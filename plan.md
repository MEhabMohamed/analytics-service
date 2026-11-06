# `analytics-service` Scope Plan: Core Slice vs. Extended Modules

This document outlines the foundational vertical slice and the full suite of analytics capabilities implemented in `analytics-service`.

---

## Part 1: Foundational Vertical Slice (Core Video Scope)

1. **Infrastructure & Layering (`pkg/` + `lib/` + `cmd/api/`)**:
   - `pkg/mongo`: Pure connection/disconnection wrapper.
   - `pkg/messaging`: Pure `amqp091-go` broker wrapper with DLX/DLQ and manual `Ack`/`Nack`.
   - `pkg/httpclient`: `net/http` client with timeout, JSON decoding, and retry-on-5xx.
   - `lib/config`: Environment config loaded via `caarlos0/env/v11`.
   - `lib/logger` & `lib/middleware`: Structured JSON `slog` logging + `X-CorrelationId` propagation.
   - `lib/errors` & `lib/http`: `AppError` + `Wrap` middleware and `{"success": true, "data": ...}` envelopes.
   - `lib/auth`, `lib/coreclient`, & `lib/rbac`: HS256 JWT verification + read-through role permission cache backed by `core-service`.
   - `lib/coreevents`: Generic event envelope parser + `EventDeduper` interface.
2. **Core Aggregate & Endpoint**:
   - `agg_restaurant_day` collection + `event_ids` idempotency collection (unique `event_id`, 7-day TTL).
   - `order.placed` event handler incrementing `orders_count` and `revenue_sum`.
   - `GET /api/v1/analytics/restaurants/{restaurantId}/days?from=&to=` with service-derived `avgOrderMinor`.

---

## Part 2: Extended Analytics Suite (Implemented End-to-End)

1. **Three Additional Aggregate Collections**:
   - `agg_branch_day` — per-branch daily orders, deliveries, rejections, cancellations, revenue, and delivery duration (`delivery_ms_sum`, `delivery_ms_count`).
   - `agg_product_day` — per-product daily quantity sold, revenue, and order count updated via a **single `BulkWrite`** per order (zero N+1 queries).
   - `agg_platform_day` — per-region daily GMV, commission, service fees, delivery fees, online payments, payment failures, and delivery times.
2. **Additional Event Handlers**:
   - `payment.completed` & `payment.failed` — online payment volume and failure tracking.
   - `order.delivered` — associative `delivery_ms_sum` + `delivery_ms_count` and commission tracking. Safe under out-of-order delivery (`order.delivered` arriving before `order.placed`) via `$inc` + `$setOnInsert` upserts keyed on the order's placement date.
   - `order.rejected` & `order.cancelled` — failure counter rollups.
   - `rbac.permissions_changed` — consumed from `core.events` (`rbac.#`) to invalidate `lib/rbac.Cache`.
3. **Seven Additional HTTP Endpoints**:
   - `GET /api/v1/analytics/restaurants/{restaurantId}/failures?from=&to=` (derived from `agg_restaurant_day`)
   - `GET /api/v1/analytics/restaurants/{restaurantId}/delivery-avg?from=&to=` (derived from `agg_restaurant_day`)
   - `GET /api/v1/analytics/restaurants/{restaurantId}/branches/days?from=&to=&branchId=`
   - `GET /api/v1/analytics/restaurants/{restaurantId}/products/top?from=&to=&branchId=&limit=`
   - `GET /api/v1/analytics/platform/days?from=&to=&region=`
   - `GET /api/v1/analytics/platform/active-restaurants?from=&to=` (derived from `agg_restaurant_day`)
   - `GET /api/v1/analytics/platform/failures?from=&to=&region=` (derived from `agg_platform_day`)
4. **Historical Backfill CLI (`cmd/backfill-aggs/main.go`)**:
   - Keyset-paginated reader over regional Postgres shards using batch `WHERE order_id = ANY($1)` queries and MongoDB `BulkWrite` (zero N+1 queries).

