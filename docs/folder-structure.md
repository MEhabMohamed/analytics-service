# Annotated Folder Structure — `analytics-service`

```
analytics-service/
├── cmd/
│   ├── api/
│   │   └── main.go                  # Entrypoint (~15 lines). Calls lib/boot.Run(). Never edited when adding modules.
│   └── backfill-aggs/               # Standalone CLI to backfill Mongo aggregates from regional Postgres shards (keyset + BulkWrite).
│       ├── main.go                  # CLI flags, config, Mongo bootstrap, and shard loop.
│       ├── pgreader/reader.go       # Keyset-paginated batch SQL readers (FetchOrderBatch, FetchItemsForOrders, FetchTransactionsForOrders).
│       └── flusher/flush.go         # Batch deduplication + BulkWrite flush to all 4 MongoDB aggregate collections.
│
├── pkg/                             # PURE PROVIDERS: zero imports from lib/ or app/, no env vars, no collection names.
│   ├── mongo/
│   │   └── client.go                # Connect(ctx, uri, dbName) and Disconnect(ctx). Knows nothing about agg_*_day.
│   ├── messaging/
│   │   ├── types.go                 # Broker interface, Topology, Delivery, HandlerFunc.
│   │   ├── amqp.go                  # AMQPBroker struct, Connect, DeclareTopology, Close.
│   │   └── amqp_pubsub.go           # Consume (goroutine channel reader) and Publish.
│   └── httpclient/
│       └── client.go                # net/http wrapper with timeout, JSON unmarshaling, and exponential-backoff retry on 5xx.
│
├── lib/                             # APP-AWARE GLUE: imports pkg/ and env, NEVER imports app/<module>/*.
│   ├── boot/
│   │   └── boot.go                  # Explicit constructor DI wiring of all singletons + graceful shutdown on SIGINT/SIGTERM.
│   ├── config/
│   │   └── env.go                   # Config struct with `env:"..."` tags parsed via caarlos0/env/v11.
│   ├── appcontext/
│   │   └── context.go               # Typed context.Context keys for JWT Claims and CorrelationID (Go analogue of express.d.ts).
│   ├── logger/
│   │   └── logger.go                # stdlib log/slog JSON logger + FromContext(ctx) correlationId enrichment.
│   ├── errors/
│   │   ├── apperror.go              # AppError struct (Code, Status, Message) + cross-cutting sentinels.
│   │   └── handler.go               # Wrap(logger, handler) adapter converting returned error into {"success": false, "error": ...}.
│   ├── http/
│   │   └── response.go              # SendSuccess and SendPaginated envelope helpers.
│   ├── middleware/
│   │   └── correlation.go           # X-CorrelationId propagation and structured HTTP access logging.
│   ├── auth/
│   │   ├── jwt.go                   # HS256 JWT parser using ACCESS_SECRET.
│   │   ├── middleware.go            # Authenticate, RequireAdmin, RequireRestaurantMember, RequireBranchAccess.
│   │   └── apikey.go                # Constant-time api-key header guard for internal endpoints.
│   ├── coreclient/
│   │   ├── types.go                 # CoreEnvelope and RolePermissionsResponse wire types.
│   │   ├── client.go                # Sync HTTP client to core-service.
│   │   └── rbac.go                  # GetPermissionsByRole(/api/internal/rbac/permissions).
│   ├── rbac/
│   │   ├── cache.go                 # Thread-safe in-memory role→permissions cache with TTL and Invalidate(role).
│   │   └── middleware.go            # Require("analytics:read") middleware.
│   └── coreevents/
│       ├── payloads.go              # Envelope, EventHandler, and EventDeduper interface (dependency inversion).
│       └── consumer.go              # Generic AMQP consumer: parse -> MarkSeen dedupe -> dispatch handler -> Ack/Nack(DLQ).
│
├── app/
│   └── analytics/                   # PACKAGE analytics — parent package holding module-shared types, errors, and enums.
│       ├── enums.go                 # PermAnalyticsRead, EventOrderPlaced, CollRestaurantDay, etc.
│       ├── errors.go                # ErrInvalidDateRange, ErrValidation, etc.
│       ├── types.go                 # Inbound event inputs + query filter inputs.
│       ├── projections.go           # Service projection output rows and summaries.
│       ├── entity/                  # Plain structs with `bson` tags matching MongoDB documents.
│       │   ├── restaurant_day.entity.go
│       │   ├── branch_day.entity.go
│       │   ├── product_day.entity.go
│       │   ├── platform_day.entity.go
│       │   └── event_id.entity.go
│       ├── repository/              # ONLY place where go.mongodb.org/mongo-driver is used for queries/indexes.
│       │   ├── indexes.go           # Idempotent EnsureIndexes(ctx, db) run on boot.
│       │   ├── event_ids.repo.go    # Satisfies coreevents.EventDeduper via unique index InsertOne + batch FilterAndMarkBatchSeen.
│       │   ├── restaurant_day.repo.go
│       │   ├── restaurant_day_queries.repo.go
│       │   ├── branch_day.repo.go
│       │   ├── product_day.repo.go  # Uses BulkWrite for multi-item order.placed events (zero N+1).
│       │   ├── platform_day.repo.go
│       │   └── platform_day_queries.repo.go
│       ├── dto/                     # Wire-format validation and Response DTO mappers.
│       │   ├── analytics.request.go
│       │   ├── analytics.response.go
│       │   └── platform.response.go
│       ├── service/                 # Pure business logic divided by responsibility:
│       │   ├── analytics.service.go  # Service struct, constructor New(), and shared date/currency helpers.
│       │   ├── events.service.go     # Inbound RabbitMQ event write projections.
│       │   ├── restaurant.service.go # Restaurant, branch, and product read projections.
│       │   └── platform.service.go   # Platform-wide admin read projections.
│       ├── eventhandlers/
│       │   └── handlers.go          # Maps RabbitMQ event types to service methods and RBAC cache invalidation.
│       └── controller/
│           ├── analytics.controller.go
│           └── routes.go            # Mounts all 8 endpoints under /api/v1/analytics.
│
├── play/                            # GITIGNORED dev aids for local E2E testing without Postgres/order-service.
│   ├── mock-core/main.go
│   ├── mint-jwt/main.go
│   ├── publish-test/main.go
│   └── check-mongo/main.go
│
└── docs/                            # Architectural & teaching documentation.
```

---

## Why This Layering Matters

1. **`pkg/` has zero domain coupling**: `pkg/mongo`, `pkg/messaging`, and `pkg/httpclient` can be copied verbatim into another Go microservice without changing a single line.
2. **`lib/` never imports `app/`**: To let `lib/coreevents/consumer.go` deduplicate events using MongoDB without importing `app/analytics/repository`, `lib/coreevents` defines the 1-method `EventDeduper` interface, and `repository.EventIDsRepo` satisfies it implicitly via Go duck typing.
3. **`app/analytics/` parent package holds shared types**: Subpackages (`service`, `controller`, `dto`, `repository`, `eventhandlers`) all import `quickbite/analytics-service/app/analytics` for `OnOrderPlacedInput`, `RestaurantDayRow`, and `ErrInvalidDateRange`, keeping `service/analytics.service.go` focused strictly on business methods.

