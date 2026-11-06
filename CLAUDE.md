# CLAUDE.md — Analytics Service Guidelines

These rules apply to the **`analytics-service`** microservice of the **QuickBite** platform. They mirror the architectural conventions of `core-service` and `order-service` (Node/TypeScript) translated into idiomatic **Go**.

---

## 1. Mission of this service

`analytics-service` owns per-day rollups of order, payment, and delivery activity:

- **Consumes** domain events from `order-service` (`order.events` exchange) and `core-service` (`core.events` exchange) via RabbitMQ.
- **Upserts** day-grained aggregates into MongoDB (`agg_restaurant_day`, `agg_branch_day`, `agg_product_day`, `agg_platform_day`).
- **Serves** read-only HTTP endpoints under `/api/v1/analytics/...`.

It does **not** own users, restaurants, products, orders, or payments. It does **not** write to any operational data store. It does **not** emit events.

---

## 2. Locked tech stack — do not deviate

| Concern        | Choice                                              |
| -------------- | --------------------------------------------------- |
| Runtime        | Go 1.21+                                            |
| HTTP router    | `github.com/go-chi/chi/v5`                          |
| DB             | `go.mongodb.org/mongo-driver` (official, v1)        |
| Messaging      | `github.com/rabbitmq/amqp091-go`                    |
| Config         | `github.com/caarlos0/env/v11` (struct tags)         |
| Logger         | stdlib `log/slog`                                   |
| Validation     | `github.com/go-playground/validator/v10`            |
| JWT            | `github.com/golang-jwt/jwt/v5`                      |
| UUID           | `github.com/google/uuid`                            |
| DI             | **NO framework.** Explicit constructor wiring in `lib/boot/boot.go`. |

- **Forbidden:** GORM / Ent / any ODM. Repositories use the official `mongo-driver` directly with typed structs and `bson` tags.
- **Forbidden:** DDL migrations. Mongo is schemaless. Indexes live in `app/analytics/repository/indexes.go` and are created idempotently on boot via `EnsureIndexes`.

---

## 3. Folder structure

```
analytics-service/
├── cmd/
│   ├── api/main.go                # ~10 lines: just calls lib/boot.Run()
│   └── backfill-aggs/             # Zero-N+1 keyset-paginated Postgres-to-Mongo backfill CLI
│       ├── main.go                # CLI flags, config, Mongo bootstrap, and shard loop
│       ├── pgreader/reader.go     # Batch SQL readers (FetchOrderBatch, FetchItemsForOrders, FetchTransactionsForOrders)
│       └── flusher/flush.go       # Batch dedupe + 4-collection BulkWrite flush
├── pkg/                           # framework-free, NO imports from lib/ or app/, NO app-specific knowledge
│   ├── mongo/client.go            # Connect/Disconnect ONLY — knows nothing about collections
│   ├── messaging/{types.go, amqp.go, amqp_pubsub.go} # Broker interface + amqp091 impl
│   └── httpclient/client.go               # net/http wrapper: timeout, JSON, retry-on-5xx
├── lib/                           # app-aware glue: env, middleware
│   ├── boot/boot.go               # wires every singleton; main.go just calls Run()
│   ├── config/env.go              # struct + Load() — equivalent of the zod schema
│   ├── logger/logger.go           # slog wrapper + FromContext
│   ├── appcontext/context.go      # ctx keys: claims, correlation_id (≈ express.d.ts)
│   ├── errors/{apperror.go, handler.go}   # AppError + Wrap(handler) middleware
│   ├── http/response.go           # SendSuccess, SendPaginated
│   ├── middleware/correlation.go  # Correlation + AccessLog
│   ├── auth/{jwt.go, middleware.go, apikey.go}
│   ├── rbac/{cache.go, middleware.go}     # in-process cache + Require("perm")
│   ├── coreclient/{client.go, rbac.go, types.go}
│   └── coreevents/{consumer.go, payloads.go}  # generic only; payloads.go holds Envelope + EventHandler + EventDeduper
├── app/
│   └── analytics/                          # PACKAGE analytics — shared module types live here
│       ├── types.go                        # Inbound event inputs + DateRangeQuery
│       ├── projections.go                  # Service projection output rows and summaries
│       ├── errors.go                       # var Err… = apperr.New(...)
│       ├── enums.go                        # const PermAnalyticsRead = "analytics:read"
│       ├── entity/                         # plain structs + bson tags
│       ├── repository/                     # ONLY place mongo-driver appears
│       │   └── indexes.go                  # EnsureIndexes for this module's collections (NOT in pkg/mongo)
│       ├── service/                        # Divided into focused files (<220 lines each):
│       │   ├── analytics.service.go        # Service struct, New(), shared helpers
│       │   ├── events.service.go           # Inbound RabbitMQ event write projections
│       │   ├── restaurant.service.go       # Restaurant, branch, and product read projections
│       │   └── platform.service.go         # Platform-wide admin read projections
│       ├── controller/{analytics.controller.go, routes.go}
│       ├── dto/{analytics.request.go, analytics.response.go, platform.response.go}
│       └── eventhandlers/handlers.go       # event type → service method map
├── play/                          # GITIGNORED — dev aids only (mock-core, mint-jwt, publish-test, check-mongo)
└── docs/
```

---

## 4. Layering rules

```
app/  → may import lib, pkg
lib/  → may import pkg, env; may NOT import app/<module>/*
pkg/  → no imports from lib or app, no env, no globals, NO app-specific knowledge
```

- `pkg/mongo` knows how to connect; it does **not** know collection names (`agg_restaurant_day`, etc.). Collection names and indexes live in `app/analytics/repository/indexes.go`.
- When `lib/` needs behavior implemented in `app/`, invert the dependency with a small interface in `lib/` (e.g., `coreevents.EventDeduper` satisfied implicitly by `repository.EventIDsRepo`).
- Never declare `type X struct{...}` inside `*.service.go`, `*.controller.go`, or `*.repo.go`. Shared module types belong in `app/analytics/types.go`.

---

## 5. Naming conventions

- **Go packages**: lowercase, single word (`analytics`, `repository`, `service`, `controller`, `dto`, `entity`).
- **Go identifiers**: `PascalCase` for exported, `camelCase` for unexported.
- **MongoDB collections & fields**: `snake_case` (`agg_restaurant_day`, `restaurant_id`, `orders_count`, `revenue_sum`).
- **JSON wire fields**: `camelCase` (`ordersCount`, `revenueMinor`, `avgOrderMinor`).
- **Money**: `int64` minor units (e.g. piasters/cents) everywhere. Never float/decimal.
- **Dates & Timestamps**: Day buckets as `"YYYY-MM-DD"` UTC; timestamps as ISO-8601 UTC strings on the wire and `time.Time` in BSON.

---

## 6. Response shape & error handling

- **Success envelope**: `{"success": true, "data": ...}`
- **Error envelope**: `{"success": false, "error": {"code": "...", "message": "..."}}`
- Every HTTP response goes through a **Response DTO struct** in `app/analytics/dto/*.response.go`. Never serialize entity structs directly.
- Empty slices in DTOs must be initialized with `make([]DTO, 0)` so JSON encodes `[]` instead of `null`.
- Services and controllers return `error`. `errors.Wrap(logger, handler)` renders the envelope.

---

## 7. Database & performance rules (Zero N+1)

1. **No N+1 queries or I/O inside `for` loops**:
   - **Strict rule**: Never place database queries (`InsertOne`, `UpdateOne`, `Find`, `UPDATE ... WHERE id = $1`), cache round-trips (`exists`, `sismember`), or sequential broker publishes inside a `for` loop (`for (const row of rows)` in TS or `for _, r := range rows` in Go).
   - Always transform/collect items in memory first, then execute a single batch operation outside the loop (`WHERE id IN (...)`, multi-row `INSERT`, `Promise.allSettled`, or MongoDB `BulkWrite`).
   - Line-item upserts (`agg_product_day`) MUST use a single `collection.BulkWrite` per order event, never a loop of `UpdateOne`.
   - Multi-day / multi-restaurant rollups (`products/top`, `platform/active-restaurants`) MUST use a single MongoDB aggregation pipeline or indexed range query.
   - `cmd/backfill-aggs` MUST batch-fetch child rows (`WHERE order_id = ANY($1)`), batch-dedupe `event_ids` via `FilterAndMarkBatchSeen` (`$in` + `BulkWrite`), and flush all 4 aggregate collections via `BulkWrite`.
2. **Associative counters**: Store averages as `*_sum` + `*_count` (`delivery_ms_sum`, `delivery_ms_count`) so out-of-order events (`order.delivered` arriving before `order.placed`) and replays merge commutatively via `$inc`.
3. **Every query backed by an index**: All queries in `repository/*.repo.go` are covered by compound indexes in `repository/indexes.go`.

---

## 8. Cross-cutting infra

- **Auth**: HS256 JWT verified with `ACCESS_SECRET`. Reads `access_token` cookie first, then `Authorization: Bearer`.
- **RBAC**: Looks up role permissions via `core-service` (`GET /api/internal/rbac/permissions?role=...` with `api-key` header), cached in memory with TTL (default 5m) and invalidated on `rbac.permissions_changed` events.
- **Idempotency**: Every inbound RabbitMQ event passes through `EventDeduper.MarkSeen(eventId)` backed by a unique index on `event_ids.event_id` (7-day TTL on `received_at`). Duplicate → `Ack()` and skip. Unknown event type → warn + `Ack()` and skip.

---

## 9. What to avoid

- ❌ Importing `go.mongodb.org/mongo-driver` outside `pkg/mongo` or `app/*/repository`.
- ❌ Putting collection names in `pkg/mongo`.
- ❌ Declaring inline `type X struct` in service, controller, or repository files.
- ❌ Using `bson.M` for compound index definitions or `$sort` stages (use `bson.D` to preserve key order).
- ❌ Returning `nil` slices in JSON responses (initialize with `make([]T, 0)`).
- ❌ Looping `UpdateOne` per line item instead of `BulkWrite`.

---

## 10. Reference docs (`docs/`)

- `docs/folder-structure.md` — Annotated directory tree and package boundaries.
- `docs/system-design.md` — Platform topology, sync vs. async flows, failure modes, and why Mongo dedupe beats Redis `SETNX` here.
- `docs/api-contracts.md` — Inbound event schemas, HTTP endpoints, and error codes.
- `docs/node-to-go-mapping.md` — Side-by-side Node/TS to Go translation guide.
- `docs/ai-prompts.md` — Prompt templates for AI-assisted development.
- `docs/implementation-plan.md` — Phased build order and acceptance checks.

---

## 11. Out of scope

- Owning or mutating operational entities (orders, payments, users, restaurants, branches, products).
- Publishing outbound RabbitMQ events.
- Real-time WebSocket push (handled by `order-service`).

