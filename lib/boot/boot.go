// Package boot wires every singleton in analytics-service and manages the process lifecycle.
//
// TS vs Go Syntax Note:
//   - In `order-service`, `src/lib/di/container.ts` registers classes in `tsyringe` (`container.registerSingleton`),
//     `src/app.ts` builds the Express app, and `src/server.ts` starts HTTP + RabbitMQ + graceful shutdown.
//   - In Go, reflection DI containers are an anti-pattern. Instead, `boot.Run()` performs explicit
//     constructor wiring (`repo := repository.New...(db)`, `svc := service.New(repo, ...)`), so missing
//     dependencies are caught by the Go compiler at build time rather than crashing at runtime!
//   - Notice `signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`: this is Go's
//     stdlib equivalent of `process.on("SIGINT", shutdown); process.on("SIGTERM", shutdown);`.
package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"quickbite/analytics-service/app/analytics/controller"
	"quickbite/analytics-service/app/analytics/eventhandlers"
	"quickbite/analytics-service/app/analytics/repository"
	"quickbite/analytics-service/app/analytics/service"
	"quickbite/analytics-service/lib/config"
	"quickbite/analytics-service/lib/coreclient"
	"quickbite/analytics-service/lib/coreevents"
	httplib "quickbite/analytics-service/lib/http"
	"quickbite/analytics-service/lib/logger"
	"quickbite/analytics-service/lib/middleware"
	"quickbite/analytics-service/lib/rbac"
	"quickbite/analytics-service/pkg/messaging"
	pkgmongo "quickbite/analytics-service/pkg/mongo"
)

type healthStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// Run loads configuration, connects to MongoDB and RabbitMQ, starts consumers and HTTP server,
// and blocks until SIGINT/SIGTERM triggers graceful shutdown.
func Run() error {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err.Error())
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 1. Connect MongoDB & ensure indexes idempotently
	mongoClient, err := pkgmongo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Error("mongo connection failed", "error", err.Error())
		return err
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(shutCtx)
	}()

	if err := repository.EnsureIndexes(ctx, mongoClient.DB()); err != nil {
		log.Error("mongo ensure indexes failed", "error", err.Error())
		return err
	}
	log.Info("mongo connected", "db", cfg.MongoDB)

	// 2. Construct repositories & domain service (explicit DI)
	eventIDsRepo := repository.NewEventIDsRepo(mongoClient.DB())
	restaurantRepo := repository.NewRestaurantDayRepo(mongoClient.DB())
	branchRepo := repository.NewBranchDayRepo(mongoClient.DB())
	productRepo := repository.NewProductDayRepo(mongoClient.DB())
	platformRepo := repository.NewPlatformDayRepo(mongoClient.DB())

	analyticsSvc := service.New(restaurantRepo, branchRepo, productRepo, platformRepo)

	// 3. Construct core-service HTTP client & in-process RBAC cache
	coreClient := coreclient.New(cfg.CoreServiceBaseURL, cfg.CoreInternalAPIKey)
	rbacCache := rbac.NewCache(coreClient, time.Duration(cfg.RBACCacheTTLSec)*time.Second)

	// 4. Connect RabbitMQ & start consumers (order.events + core.events); non-fatal at boot with background retry
	broker := messaging.NewAMQPBroker(cfg.RabbitURL)
	defer func() {
		_ = broker.Close()
	}()

	orderConsumer := coreevents.NewConsumer(broker, eventIDsRepo, log)
	eventhandlers.RegisterOrderEventHandlers(orderConsumer, analyticsSvc)
	orderTop := messaging.Topology{
		Exchange:           cfg.OrderEventsExchange,
		Queue:              cfg.OrderEventsQueue,
		BindingKeys:        cfg.OrderEventsBindings,
		DeadLetterExchange: cfg.OrderEventsDLX,
		DeadLetterQueue:    cfg.OrderEventsDLQ,
		Prefetch:           cfg.RabbitPrefetch,
	}

	coreConsumer := coreevents.NewConsumer(broker, eventIDsRepo, log)
	eventhandlers.RegisterCoreEventHandlers(coreConsumer, rbacCache, log)
	coreTop := messaging.Topology{
		Exchange:           cfg.CoreEventsExchange,
		Queue:              cfg.CoreEventsQueue,
		BindingKeys:        cfg.CoreEventsBindings,
		DeadLetterExchange: cfg.CoreEventsDLX,
		DeadLetterQueue:    cfg.CoreEventsDLQ,
		Prefetch:           cfg.RabbitPrefetch,
	}

	startMessaging(ctx, log, broker, orderConsumer, orderTop, coreConsumer, coreTop)

	// 5. Build Chi HTTP router & mount routes
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.Correlation)
	r.Use(middleware.AccessLog(log))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httplib.SendSuccess(w, http.StatusOK, healthStatus{
			Status:  "ok",
			Service: "analytics-service",
		})
	})

	analyticsCtrl := controller.New(analyticsSvc)
	controller.RegisterRoutes(r, analyticsCtrl, cfg.AccessSecret, rbacCache, log)

	// 6. Start HTTP server & wait for shutdown signal
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listening", "port", cfg.Port, "env", cfg.Env)
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received, draining connections")
	case serveErr := <-errCh:
		if serveErr != nil {
			log.Error("http server failed", "error", serveErr.Error())
			return serveErr
		}
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}

func startMessaging(
	ctx context.Context,
	log *slog.Logger,
	broker *messaging.AMQPBroker,
	orderConsumer *coreevents.Consumer,
	orderTop messaging.Topology,
	coreConsumer *coreevents.Consumer,
	coreTop messaging.Topology,
) {
	tryStart := func(orderStarted, coreStarted *bool) error {
		if err := broker.Connect(ctx); err != nil {
			return fmt.Errorf("rabbit connect: %w", err)
		}
		if !*orderStarted {
			if err := orderConsumer.Start(ctx, orderTop); err != nil {
				return fmt.Errorf("start order events consumer: %w", err)
			}
			*orderStarted = true
		}
		if !*coreStarted {
			if err := coreConsumer.Start(ctx, coreTop); err != nil {
				return fmt.Errorf("start core events consumer: %w", err)
			}
			*coreStarted = true
		}
		return nil
	}

	var orderStarted, coreStarted bool
	if err := tryStart(&orderStarted, &coreStarted); err == nil {
		log.Info("rabbit connected and event consumers started",
			"orderQueue", orderTop.Queue,
			"coreQueue", coreTop.Queue,
		)
		return
	} else {
		log.Warn("rabbit not reachable at boot, will retry in background", "error", err.Error())
	}

	go func() {
		delay := 2 * time.Second
		const maxDelay = 30 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if err := tryStart(&orderStarted, &coreStarted); err == nil {
				log.Info("rabbit connected and event consumers started",
					"orderQueue", orderTop.Queue,
					"coreQueue", coreTop.Queue,
				)
				return
			} else {
				log.Warn("rabbit reconnect attempt failed", "error", err.Error(), "retryIn", delay.String())
			}
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}()
}

