# System Design — `analytics-service`

---

## 1. Platform Topology

```
┌──────────────────────┐         Sync HTTP (RBAC lookup)         ┌────────────────────────┐
│                      │ ◄────────────────────────────────────── │                        │
│     core-service     │   GET /api/internal/rbac/permissions    │   analytics-service    │
│  (Node + TS + PG)    │                                         │   (Go + Chi + Mongo)   │
│                      │ ───────► [Exchange: core.events] ─────► │                        │
└──────────────────────┘          rbac.permissions_changed       └───────────┬────────────┘
                                                                             │
┌──────────────────────┐                                                     │
│                      │          [Exchange: order.events]                   │ Reads / Upserts
│    order-service     │          order.placed, order.delivered,             ▼
│ (Node + Sharded PG)  │ ───────► order.rejected, payment.completed ──► ┌─────────────────┐
│  + Outbox Drainer    │          (Queue: analytics-service.order-events)│     MongoDB     │
└──────────────────────┘                                                 │ (agg_*_day,     │
                                                                         │  event_ids)     │
                                                                         └─────────────────┘
```

---

## 2. Synchronous vs. Asynchronous Flows

### Asynchronous Write Path (RabbitMQ → MongoDB)
1. **Transactional Outbox in `order-service`**: When an order is placed, delivered, rejected, or paid, `order-service` writes the domain row and an `events_outbox` row in the **same regional Postgres transaction**.
2. **Batch Outbox Drain**: `order-service`'s worker claims undispatched rows with `FOR UPDATE SKIP LOCKED`, publishes them to the `order.events` topic exchange, and marks the batch dispatched in a single bulk `UPDATE ... WHERE id IN (...)` query (zero N+1).
3. **Consumption & Deduplication in `analytics-service`**:
   - `analytics-service` consumes from `analytics-service.order-events` (`order.#`, `payment.#`).
   - Before invoking any handler, `Consumer` calls `EventIDsRepo.MarkSeen(ctx, eventId, eventType)` which performs `InsertOne` into `event_ids` (backed by unique index `uq_event_id`).
   - If `InsertOne` returns a duplicate-key error, the event was already processed: the consumer `Ack`s and skips immediately.
4. **Associative Rollup Upserts**:
   - Handlers update `agg_restaurant_day`, `agg_branch_day`, `agg_product_day` (via `BulkWrite`), and `agg_platform_day` using `$inc`, `$set`, and `$setOnInsert` with `Upsert: true`.
   - After all Mongo updates succeed, the consumer `Ack`s the delivery.

### Synchronous Read Path (HTTP Client → `analytics-service` → `core-service` / MongoDB)
1. Client sends `GET /api/v1/analytics/restaurants/42/days?from=2026-11-01&to=2026-11-06` with `access_token` cookie or `Authorization: Bearer <jwt>`.
2. `auth.Authenticate` verifies the HS256 signature using `ACCESS_SECRET` and populates `appcontext.Claims`.
3. `auth.RequireRestaurantMember("restaurantId")` checks `claims.RestaurantID == 42` (`system_admin` bypasses).
4. `rbac.Require("analytics:read")` checks the in-process `rbac.Cache` for `claims.RestaurantRole`. On cache miss, it calls `core-service` `GET /api/internal/rbac/permissions?role=...` (with retry-on-5xx) and caches the result for 5 minutes.
5. `AnalyticsService` queries MongoDB using the compound index `(restaurant_id, date)`, derives computed metrics (`avgOrderMinor = revenue_sum / orders_count`, `avgDeliveryMs = delivery_ms_sum / delivery_ms_count`, `failureRateBps`), maps to Response DTOs, and returns the envelope.

---

## 3. Why MongoDB `event_ids` Dedupe Instead of Redis `SETNX`?

In `order-service`, inbound events from `core-service` (`product.price.changed`, `branch.updated`) are deduplicated with Redis `SETNX` (`core-events:dedupe:<eventId>`, 24h TTL). Why does `analytics-service` use a MongoDB collection (`event_ids`) with a unique index instead?

1. **Idempotent Cache Eviction vs. Non-Idempotent Counter Increment (`$inc`)**:
   - In `order-service`, every event handler performs `redis.del("core:branch:123")`. Deleting a cache key twice is naturally idempotent — if Redis restarts and loses the `SETNX` key, re-running `redis.del` is completely harmless.
   - In `analytics-service`, handlers perform `$inc: {orders_count: 1, revenue_sum: 2500}`. Incrementing a counter is **not** naturally idempotent! If a volatile cache lost the dedupe key and RabbitMQ redelivered an event, revenue and order counts would be permanently double-counted.
2. **Same Durability Boundary as the Aggregates**:
   - Storing `event_ids` in the same MongoDB deployment as `agg_*_day` ensures the deduplication state shares the exact same availability, backup, and persistence lifecycle as the rollup documents—and avoids introducing Redis as an extra operational dependency in `analytics-service`.
3. **Atomic Uniqueness + Automatic Cleanup**:
   - MongoDB's unique index on `event_id` makes `InsertOne` an atomic compare-and-set operation across multiple `analytics-service` replicas, while the TTL index on `received_at` (`604800`s = 7 days) bounds collection size automatically.

---

## 4. Handling Out-of-Order Events (`order.delivered` Before `order.placed`)

Because `order-service` is region-sharded and runs concurrent worker ticks, `order.delivered` or `order.rejected` can occasionally arrive at `analytics-service` before `order.placed`:

- **Shared Placement Date Key**: `order.delivered` and `order.rejected` payloads carry `placedAt` alongside `deliveredAt`/`occurredAt`. `analytics-service` always buckets by the UTC date of `placedAt`.
- **Commutative `$inc` + `$setOnInsert` Upserts**:
  - If `order.delivered` arrives first, `UpdateOne(..., {Upsert: true})` creates the day document with `delivered_count: 1, delivery_ms_sum: 1800000, delivery_ms_count: 1` and initializes `orders_count: 0, revenue_sum: 0` via `$setOnInsert`.
  - When `order.placed` arrives second, its `$inc: {orders_count: 1, revenue_sum: 2500}` adds onto the existing document without touching `delivered_count` or `delivery_ms_sum`. The final document state is identical regardless of arrival order.

---

## 5. Failure-Mode Table

| Failure Scenario | System Behavior | Recovery / Guarantee |
| :--- | :--- | :--- |
| **Malformed JSON / missing `eventId` on AMQP** | Consumer logs error and calls `Nack(requeue=false)`. | Routed immediately to `*.dlq` via DLX; never blocks the queue. |
| **Unknown `eventType` (e.g. `order.accepted`)** | Consumer logs warning and calls `Ack()`. | Safely ignored; does not pollute the DLQ. |
| **Duplicate delivery of already-seen `eventId`** | `EventIDsRepo.MarkSeen` hits `E11000` dup-key → returns `fresh=false`. | Consumer `Ack`s and skips; aggregates remain untouched. |
| **MongoDB transient error during `$inc` upsert** | Handler returns error → Consumer calls `Nack(requeue=false)` → DLQ. | Operator can replay DLQ; if `MarkSeen` succeeded before a partial write, `cmd/backfill-aggs` reconciles from Postgres truth. |
| **`core-service` down during RBAC lookup** | `pkg/httpclient` retries 3 times with backoff; on failure, `rbac.Cache` serves stale permissions for up to 1 hour. | Active roles keep working through brief `core-service` restarts. |
| **RabbitMQ down when `order-service` places order** | Outbox row commits in Postgres; drainer marks attempt failed and bails out of batch. | Next worker tick drains the backlog once RabbitMQ recovers. |

