# Analytics Service — cURL Commands for Postman

All `analytics-service` endpoints authenticate using the **`access_token`** issued by `core-service` (`POST http://localhost:3000/api/auth/login`) — either automatically via Postman's `access_token` cookie on `localhost` or via `Authorization: Bearer {{access_token}}`. No hardcoded tokens are used.

---

## 0. Login via `core-service` (Sets `access_token` Cookie & Extracts `{{access_token}}`)

When you run this login request in Postman, `core-service` sets the `access_token` HTTP-only cookie for `localhost` (which Postman automatically sends to `http://localhost:5000`) and returns `data.accessToken` in the response body:

```bash
curl --location 'http://localhost:3000/api/auth/login' \
--header 'Content-Type: application/json' \
--data-raw '{
  "email": "owner@example.com",
  "password": "your_password"
}'
```

---

## 1. Health Check (`GET /health`)

```bash
curl --location 'http://localhost:5000/health'
```

---

## 2. Restaurant Endpoints (Uses `access_token` from Login)

### 2.1 Daily Orders & Revenue Rollup (`GET /api/v1/analytics/restaurants/:restaurantId/days`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/restaurants/42/days?from=2026-10-01&to=2026-10-31' \
--header 'Authorization: Bearer {{access_token}}'
```

### 2.2 Restaurant Order Failures & Failure Rate (`GET /api/v1/analytics/restaurants/:restaurantId/failures`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/restaurants/42/failures?from=2026-10-01&to=2026-10-31' \
--header 'Authorization: Bearer {{access_token}}'
```

### 2.3 Restaurant Average Delivery Time (`GET /api/v1/analytics/restaurants/:restaurantId/delivery-avg`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/restaurants/42/delivery-avg?from=2026-10-01&to=2026-10-31' \
--header 'Authorization: Bearer {{access_token}}'
```

### 2.4 Daily Rollups per Branch — All Branches (`GET /api/v1/analytics/restaurants/:restaurantId/branches/days`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/restaurants/42/branches/days?from=2026-10-01&to=2026-10-31' \
--header 'Authorization: Bearer {{access_token}}'
```

### 2.5 Daily Rollups per Branch — Filtered by `branchId` (`GET /api/v1/analytics/restaurants/:restaurantId/branches/days?branchId=7`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/restaurants/42/branches/days?from=2026-10-01&to=2026-10-31&branchId=7' \
--header 'Authorization: Bearer {{access_token}}'
```

### 2.6 Top Selling Products by Revenue (`GET /api/v1/analytics/restaurants/:restaurantId/products/top`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/restaurants/42/products/top?from=2026-10-01&to=2026-10-31&branchId=7&limit=10' \
--header 'Authorization: Bearer {{access_token}}'
```

---

## 3. Platform-Wide Admin Endpoints (Uses `system_admin` `access_token` from Login)

### 3.1 Platform Daily GMV, Commission, & Fees (`GET /api/v1/analytics/platform/days`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/platform/days?from=2026-10-01&to=2026-10-31&region=eg' \
--header 'Authorization: Bearer {{access_token}}'
```

### 3.2 Platform Active Restaurants per Day (`GET /api/v1/analytics/platform/active-restaurants`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/platform/active-restaurants?from=2026-10-01&to=2026-10-31' \
--header 'Authorization: Bearer {{access_token}}'
```

### 3.3 Platform Order & Payment Failures (`GET /api/v1/analytics/platform/failures`)

```bash
curl --location 'http://localhost:5000/api/v1/analytics/platform/failures?from=2026-10-01&to=2026-10-31&region=eg' \
--header 'Authorization: Bearer {{access_token}}'
```
