// Package eventhandlers wires RabbitMQ event types to AnalyticsService methods and RBAC cache invalidation.
//
// TS vs Go Syntax Note:
//   - In order-service (`core-events.handlers.ts`), handlers are registered via closures calling
//     methods on a DI-resolved service: `registerHandler("product.stock.changed", async (p) => ...)`.
//   - In Go, `RegisterOrderEventHandlers` and `RegisterCoreEventHandlers` register typed closures
//     on `*coreevents.Consumer` that unmarshal `env.Payload` (`json.RawMessage`) into strongly-typed
//     input structs from `app/analytics/types.go` and delegate to `*service.Service`.
package eventhandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/app/analytics/service"
	"quickbite/analytics-service/lib/coreevents"
	"quickbite/analytics-service/lib/rbac"
)

// RegisterOrderEventHandlers binds order.events routing keys (order.placed, payment.completed,
// order.delivered, order.rejected, order.cancelled, payment.failed) to AnalyticsService methods.
func RegisterOrderEventHandlers(consumer *coreevents.Consumer, svc *service.Service) {
	consumer.Register(analytics.EventOrderPlaced, func(ctx context.Context, env coreevents.Envelope) error {
		var in analytics.OnOrderPlacedInput
		if err := json.Unmarshal(env.Payload, &in); err != nil {
			return fmt.Errorf("unmarshal order.placed: %w", err)
		}
		if in.Region == "" {
			in.Region = env.Region
		}
		return svc.OnOrderPlaced(ctx, in)
	})

	consumer.Register(analytics.EventPaymentCompleted, func(ctx context.Context, env coreevents.Envelope) error {
		var in analytics.OnPaymentCompletedInput
		if err := json.Unmarshal(env.Payload, &in); err != nil {
			return fmt.Errorf("unmarshal payment.completed: %w", err)
		}
		if in.Region == "" {
			in.Region = env.Region
		}
		return svc.OnPaymentCompleted(ctx, in)
	})

	consumer.Register(analytics.EventPaymentFailed, func(ctx context.Context, env coreevents.Envelope) error {
		var in analytics.OnPaymentFailedInput
		if err := json.Unmarshal(env.Payload, &in); err != nil {
			return fmt.Errorf("unmarshal payment.failed: %w", err)
		}
		if in.Region == "" {
			in.Region = env.Region
		}
		return svc.OnPaymentFailed(ctx, in)
	})

	consumer.Register(analytics.EventOrderDelivered, func(ctx context.Context, env coreevents.Envelope) error {
		var in analytics.OnOrderDeliveredInput
		if err := json.Unmarshal(env.Payload, &in); err != nil {
			return fmt.Errorf("unmarshal order.delivered: %w", err)
		}
		if in.Region == "" {
			in.Region = env.Region
		}
		return svc.OnOrderDelivered(ctx, in)
	})

	consumer.Register(analytics.EventOrderRejected, func(ctx context.Context, env coreevents.Envelope) error {
		var in analytics.OnOrderTransitionInput
		if err := json.Unmarshal(env.Payload, &in); err != nil {
			return fmt.Errorf("unmarshal order.rejected: %w", err)
		}
		if in.Region == "" {
			in.Region = env.Region
		}
		return svc.OnOrderRejected(ctx, in)
	})

	consumer.Register(analytics.EventOrderCancelled, func(ctx context.Context, env coreevents.Envelope) error {
		var in analytics.OnOrderTransitionInput
		if err := json.Unmarshal(env.Payload, &in); err != nil {
			return fmt.Errorf("unmarshal order.cancelled: %w", err)
		}
		if in.Region == "" {
			in.Region = env.Region
		}
		return svc.OnOrderCancelled(ctx, in)
	})
}

// RegisterCoreEventHandlers binds core.events routing keys (rbac.permissions_changed) to RBAC cache invalidation.
func RegisterCoreEventHandlers(consumer *coreevents.Consumer, rbacCache *rbac.Cache, log *slog.Logger) {
	consumer.Register(analytics.EventRBACPermissionsChanged, func(_ context.Context, env coreevents.Envelope) error {
		var in analytics.OnRBACPermissionsChangedInput
		if len(env.Payload) > 0 {
			_ = json.Unmarshal(env.Payload, &in)
		}
		rbacCache.Invalidate(in.Role)
		log.Info("rbac.permissions_changed -> permission cache cleared", "role", in.Role)
		return nil
	})
}

