# Implementation Plan & Acceptance Checklist — `analytics-service`

---

## Phase 1: `order-service` & `core-service` Publisher & N+1 Query Hardening
- **Strict Anti-N+1 Rule**:
  - Never execute database queries, cache calls, or sequential broker publishes inside a `for` loop (`for (const row of rows)` / `for _, r := range rows`). Always collect inputs in memory and execute bulk/batch operations (`whereIn`, multi-row `INSERT`, `Promise.allSettled`, or MongoDB `BulkWrite`) outside the loop.
- **Scope**:
  - `events_outbox` per-shard migration (`20261023000100_create_events_outbox.ts`).
  - `lib/events/{event-types,types,outbox.repo,outbox-drain,jobs}.ts` in both `core-service` and `order-service`: replace the `for (const row of rows)` sequential publish + per-row `markDispatched`/`markFailed` loop with concurrent `Promise.allSettled(rows.map(...))` + bulk `markDispatchedBatch` and `markFailedBatch` (`WHERE id IN (...)`).
  - Multi-row `bulkCreateTransactionsIdempotent` in `SettlementService`, batched `insertOutboxEvents` in `KashierWebhookService`, and concurrent candidate Redis checks in `AssignmentService.findCandidates`.
- **Acceptance Check**:
  - Placing an order writes an `order.placed` outbox row in the same DB transaction and the worker drains batches to RabbitMQ with a single bulk `UPDATE` and zero I/O-in-`for`-loop patterns.

---

## Phase 2: `pkg/` & `lib/` Foundation in `analytics-service`
- **Scope**:
  - `pkg/mongo`, `pkg/messaging`, `pkg/httpclient` (framework-free, zero app imports).
  - `lib/config`, `lib/appcontext`, `lib/logger`, `lib/errors`, `lib/http`, `lib/middleware`.
- **Acceptance Check**:
  - `go build ./pkg/... ./lib/...` compiles cleanly; `pkg/` has zero imports from `lib/` or `app/`.

---

## Phase 3: Cross-Cutting Auth, RBAC Read-Through Cache, & Generic Consumer
- **Scope**:
  - `lib/auth` (HS256 JWT verification from `access_token` cookie or `Authorization: Bearer`, `RequireRestaurantMember`, `RequireBranchAccess`, `RequireAdmin`).
  - `lib/coreclient` & `lib/rbac` (HTTP permission fetch from `core-service` + thread-safe TTL cache).
  - `lib/coreevents` (Envelope parser + `EventDeduper` interface + DLQ routing).
- **Acceptance Check**:
  - Unauthenticated requests return `401 UNAUTHENTICATED`; unauthorized roles return `403 FORBIDDEN`.

---

## Phase 4: Core Vertical Slice (`agg_restaurant_day` + `order.placed` + `GET .../days`)
- **Scope**:
  - `app/analytics/{enums,errors,types}.go`
  - `entity/restaurant_day.entity.go`, `entity/event_id.entity.go`
  - `repository/indexes.go`, `repository/event_ids.repo.go`, `repository/restaurant_day.repo.go`
  - `service/analytics.service.go`, `controller/analytics.controller.go`, `controller/routes.go`, `eventhandlers/handlers.go`
- **Acceptance Check**:
  - Publishing `order.placed` increments `orders_count` and `revenue_sum`; replaying the same `eventId` is skipped; `GET /api/v1/analytics/restaurants/42/days` returns `avgOrderMinor` computed in the service layer.

---

## Phase 5: Dev Test Harness (`play/`)
- **Scope**:
  - `play/mock-core`, `play/mint-jwt`, `play/publish-test`, `play/check-mongo`.
- **Acceptance Check**:
  - Full E2E verification runs cleanly using only MongoDB, RabbitMQ, `mock-core`, and `analytics-service`.

---

## Phase 6: Documentation Suite
- **Scope**:
  - `CLAUDE.md`, `README.md`, `plan.md`, and the 6 files under `docs/`.
- **Acceptance Check**:
  - All architectural rules, API contracts, and side-by-side Node-to-Go mappings are documented and accurate.

---

## Phase 7: Extended Aggregates, Event Handlers, Endpoints, & Backfill CLI
- **Scope**:
  - 3 additional collections: `agg_branch_day`, `agg_product_day` (using `BulkWrite`), `agg_platform_day`.
  - Additional event handlers: `payment.completed`, `payment.failed`, `order.delivered` (out-of-order safe), `order.rejected`, `order.cancelled`, and `rbac.permissions_changed`.
  - 7 additional endpoints: restaurant `failures`, `delivery-avg`, `branches/days`, `products/top`, and platform `days`, `active-restaurants`, `failures`.
  - `cmd/backfill-aggs/main.go` using keyset pagination + batch `ANY($1)` SQL queries + MongoDB `BulkWrite`.
- **Acceptance Check**:
  - Out-of-order `order.delivered` before `order.placed` merges cleanly; `agg_product_day` updates via `BulkWrite`; all 8 endpoints return valid Response DTO envelopes.

