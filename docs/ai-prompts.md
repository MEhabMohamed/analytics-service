# AI Prompts Guide — Directing an AI Across Languages by Anchoring to Existing Code

> **Core Meta-Lesson:** The skill you are learning isn't just Go syntax — it is **directing an AI through a non-trivial implementation in a new language by anchoring it to a codebase and architecture you already control.**

---

## 1. Golden Rules for Prompting in This Monorepo

### Rule 1: Make the AI Prove It Understood the Existing Pattern First
Never start with *"Build feature X in Go."* Instead, force a 2-step confirmation:
1. Ask the AI to read the reference files in `order-service` and `analytics-service/CLAUDE.md` and summarize the exact invariants.
2. Ask for the Node/TS analogue side-by-side with the proposed Go implementation before it writes code.

### Rule 2: Enforce Strict Package Boundaries in the Prompt
Explicitly remind the AI of the 3 non-negotiable constraints:
- No `go.mongodb.org/mongo-driver` outside `pkg/mongo` and `app/analytics/repository`.
- No inline `type X struct{...}` inside `*.service.go`, `*.controller.go`, or `*.repo.go` (shared types belong in `app/analytics/types.go`).
- Zero N+1 queries (use `BulkWrite` for multi-item writes and MongoDB aggregation pipelines for multi-row rollups).

---

## 2. Concrete Prompt Templates for Extending `analytics-service`

### Prompt 1 — Adding a New Aggregate Collection (`agg_branch_day`, `agg_product_day`, `agg_platform_day`)
```text
Read `analytics-service/CLAUDE.md`, `analytics-service/docs/node-to-go-mapping.md`, and `analytics-service/app/analytics/repository/restaurant_day.repo.go`.

Before writing any code:
1. Explain how `RestaurantDayRepo` handles associative `$inc` + `$setOnInsert` upserts and why `EnsureIndexes` lives in `app/analytics/repository/indexes.go` rather than `pkg/mongo`.
2. Propose the BSON entity struct, compound indexes (using `bson.D`), and repository methods for `agg_product_day`, ensuring `order.placed` line items are written in a single `collection.BulkWrite` call (zero N+1 queries).
Wait for my confirmation before editing files.
```

### Prompt 2 — Adding Out-of-Order Safe Event Handlers (`order.delivered`, `order.rejected`, `payment.completed`)
```text
Read `order-service/src/app/agent/service/settlement.service.ts`, `analytics-service/app/analytics/service/analytics.service.go`, and `analytics-service/app/analytics/eventhandlers/handlers.go`.

We need to handle `order.delivered`, `order.rejected`, and `payment.completed`.
1. Explain what happens if `order.delivered` arrives from RabbitMQ BEFORE `order.placed` and how `$inc` + `$setOnInsert` keyed on the UTC date of `placedAt` prevents data loss or overwritten counters.
2. Show the exact `OnOrderDeliveredInput` struct in `app/analytics/types.go`, the repository upsert, and the service method.
```

### Prompt 3 — Adding Derived Analytics Endpoints (`failures`, `delivery-avg`, `active-restaurants`)
```text
Read `analytics-service/app/analytics/controller/analytics.controller.go` and `analytics-service/app/analytics/dto/analytics.response.go`.

We want to add:
- `GET /api/v1/analytics/restaurants/{restaurantId}/failures?from=&to=`
- `GET /api/v1/analytics/restaurants/{restaurantId}/delivery-avg?from=&to=`
- `GET /api/v1/analytics/platform/active-restaurants?from=&to=`

Constraint: Do NOT create new MongoDB collections for these. Derive failure rates and average delivery times in the service layer from `agg_restaurant_day`, and use a single MongoDB `$group` aggregation pipeline for `active-restaurants` (zero N+1 queries). Make sure all empty slices in Response DTOs are initialized with `make([]DTO, 0)` so JSON returns `[]` instead of `null`.
```

### Prompt 4 — Wiring `rbac.permissions_changed` Cache Invalidation
```text
Compare `order-service/src/lib/rbac/permission-cache.service.ts` with `analytics-service/lib/rbac/cache.go` and `analytics-service/lib/boot/boot.go`.

Show how `order-service` invalidates its RBAC permission cache when `core-service` publishes `rbac.permissions_changed` to `core.events`, and wire the equivalent consumer in `analytics-service` using `lib/coreevents.Consumer` and `rbacCache.Invalidate(role)`.
```

### Prompt 5 — Building `cmd/backfill-aggs` Without N+1 Queries
```text
Read `order-service/src/migrations/20261023000010_create_orders.ts` and `analytics-service/app/analytics/repository/`.

Design `cmd/backfill-aggs/main.go` to backfill all 4 MongoDB aggregate collections from regional Postgres shards for a `-from` / `-to` date range.
Requirements:
1. Use keyset pagination (`WHERE id > $1 AND created_at >= $2 AND created_at < $3 ORDER BY id ASC LIMIT $4`), never OFFSET.
2. Batch-fetch `order_items` and `transactions` for all order IDs in the batch using `WHERE order_id = ANY($1)` (zero N+1 queries).
3. Flush updates to MongoDB using one `BulkWrite` per collection per batch, guarded by `event_ids` deduplication keys (`backfill:order:<publicId>`).
```

