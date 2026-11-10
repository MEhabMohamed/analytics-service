# QuickBite `analytics-service`

Go + MongoDB + RabbitMQ microservice that consumes domain events from `order-service` and `core-service`, maintains day-grained rollups (`agg_restaurant_day`, `agg_branch_day`, `agg_product_day`, `agg_platform_day`), and serves read-only analytics endpoints under `/api/v1/analytics/...`.

---

## Prerequisites

- **Go** 1.21+
- **MongoDB** running on `mongodb://localhost:27017`
- **RabbitMQ** running on `localhost:5672`

---

## 1. Setup `.env`

Copy `.env.example` to `.env`:

```powershell
cp .env.example .env
```

---

## 2. Three-Terminal Quickstart (Self-Contained E2E Test)

Using the dev aids in `play/`, you can test the entire service without spinning up Postgres, `core-service`, or `order-service`.

### Terminal 1 — Start `mock-core` (Internal RBAC Server)
```powershell
go run ./play/mock-core
```
Expected log:
```json
{"time":"...","level":"INFO","msg":"mock-core listening","addr":":3001"}
```

### Terminal 2 — Start `analytics-service` API & Consumers
```powershell
go run ./cmd/api
```
Expected structured JSON boot logs:
```json
{"time":"...","level":"INFO","msg":"mongo connected","db":"quickbite_analytics"}
{"time":"...","level":"INFO","msg":"rabbit connected"}
{"time":"...","level":"INFO","msg":"event consumer started","orderQueue":"analytics-service.order-events","coreQueue":"analytics-service.core-events"}
{"time":"...","level":"INFO","msg":"http listening","port":5000,"env":"development"}
```

### Terminal 3 — Publish Events, Inspect Mongo, & Call HTTP Endpoints

```powershell
# 1. Reset test collections
go run ./play/check-mongo -reset

# 2. Publish an order.placed event (restaurant=42, total=2500, date=2026-11-06)
go run ./play/publish-test -type=order.placed -event-id=11111111-1111-1111-1111-111111111111 -restaurant=42 -total=2500 -date=2026-11-06

# 3. Replay the exact same eventId (idempotency check — counters must NOT change)
go run ./play/publish-test -type=order.placed -event-id=11111111-1111-1111-1111-111111111111 -restaurant=42 -total=2500 -date=2026-11-06

# 4. Publish a second order.placed event (restaurant=42, total=1500, date=2026-11-06)
go run ./play/publish-test -type=order.placed -event-id=22222222-2222-2222-2222-222222222222 -restaurant=42 -total=1500 -date=2026-11-06

# 5. Inspect MongoDB documents
go run ./play/check-mongo

# 6. Mint a valid JWT for restaurant 42 owner and call GET /days
$TOKEN = $(go run ./play/mint-jwt -restaurant=42 -role=restaurant_user -restaurant-role=owner)
curl.exe -s -H "Authorization: Bearer $TOKEN" "http://localhost:5000/api/v1/analytics/restaurants/42/days?from=2026-11-06&to=2026-11-06"
```

Expected response:
```json
{
  "success": true,
  "data": [
    {
      "date": "2026-11-06",
      "ordersCount": 2,
      "revenueMinor": 4000,
      "currency": "EGP",
      "avgOrderMinor": 2000
    }
  ]
}
```

