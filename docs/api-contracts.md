# API & Event Contracts — `analytics-service`

---

## 1. Response Envelopes & Error Codes

### Success Envelope
```json
{
  "success": true,
  "data": { ... }
}
```

### Error Envelope
```json
{
  "success": false,
  "error": {
    "code": "ANALYTICS_INVALID_DATE_RANGE",
    "message": "'from' date must not be after 'to' date"
  }
}
```

### Canonical Error Codes

| HTTP Status | Error Code | When Returned |
| :--- | :--- | :--- |
| `400` | `VALIDATION_ERROR` | Missing/malformed `from` or `to` (`YYYY-MM-DD`), non-positive `restaurantId` / `branchId`, or `limit` outside `1..100`. |
| `400` | `ANALYTICS_INVALID_DATE_RANGE` | `from > to` (e.g. `from=2026-11-10&to=2026-11-01`). |
| `401` | `UNAUTHENTICATED` | Missing, expired, or invalid `access_token` cookie / `Authorization: Bearer` JWT. |
| `403` | `FORBIDDEN` | User lacks `analytics:read` permission, belongs to a different `restaurantId`, is not assigned to `branchId`, or is not `system_admin` on `/platform/*`. |
| `500` | `INTERNAL_ERROR` | Unexpected database or upstream failure. |

---

## 2. Inbound RabbitMQ Event Contracts

### Generic Outbox Envelope (`order.events` & `core.events`)
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440000",
  "eventType": "order.placed",
  "occurredAt": "2026-11-06T14:20:00Z",
  "aggregateType": "order",
  "aggregateId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "region": "eg",
  "payload": { ... }
}
```

### 2.1 `order.placed` (Exchange: `order.events`)
```json
{
  "orderId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "region": "eg",
  "countryCode": "eg",
  "restaurantId": 42,
  "branchId": 1,
  "customerId": 1001,
  "status": "placed",
  "paymentMethod": "cod",
  "subtotal": 2300,
  "deliveryFee": 100,
  "serviceFee": 100,
  "total": 2500,
  "currency": "EGP",
  "items": [
    {
      "productId": 10,
      "quantity": 2,
      "unitPrice": 1150,
      "lineTotal": 2300
    }
  ],
  "placedAt": "2026-11-06T14:20:00Z"
}
```

### 2.2 `payment.completed` (Exchange: `order.events`)
```json
{
  "orderId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "region": "eg",
  "restaurantId": 42,
  "branchId": 1,
  "customerId": 1001,
  "provider": "kashier",
  "providerReferenceId": "tx_98765",
  "amount": 2500,
  "currency": "EGP",
  "method": "online",
  "completedAt": "2026-11-06T14:20:05Z"
}
```

### 2.3 `order.delivered` (Exchange: `order.events`)
```json
{
  "orderId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "region": "eg",
  "countryCode": "eg",
  "restaurantId": 42,
  "branchId": 1,
  "customerId": 1001,
  "deliveryAgentId": 501,
  "total": 2500,
  "subtotal": 2300,
  "deliveryFee": 100,
  "commission": 230,
  "currency": "EGP",
  "paymentMethod": "cod",
  "placedAt": "2026-11-06T14:20:00Z",
  "deliveredAt": "2026-11-06T14:50:00Z"
}
```

### 2.4 `order.rejected` / `order.cancelled` (Exchange: `order.events`)
```json
{
  "orderId": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "region": "eg",
  "countryCode": "eg",
  "restaurantId": 42,
  "branchId": 1,
  "customerId": 1001,
  "status": "rejected",
  "currency": "EGP",
  "reason": "Kitchen closed",
  "actor": "restaurant_member",
  "placedAt": "2026-11-06T14:20:00Z",
  "occurredAt": "2026-11-06T14:22:00Z"
}
```

### 2.5 `rbac.permissions_changed` (Exchange: `core.events`)
```json
{
  "role": "branch_manager"
}
```

---

## 3. HTTP Endpoints (`/api/v1/analytics`)

### 3.1 `GET /api/v1/analytics/restaurants/{restaurantId}/days?from=YYYY-MM-DD&to=YYYY-MM-DD`
- **Auth**: JWT + `RequireRestaurantMember("restaurantId")` + RBAC `analytics:read`
- **Response (`200 OK`)**:
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

### 3.2 `GET /api/v1/analytics/restaurants/{restaurantId}/failures?from=YYYY-MM-DD&to=YYYY-MM-DD`
- **Auth**: JWT + `RequireRestaurantMember("restaurantId")` + RBAC `analytics:read`
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": {
    "totalOrders": 10,
    "totalRejected": 1,
    "totalCancelled": 1,
    "totalFailed": 2,
    "failureRateBps": 2000,
    "days": [
      {
        "date": "2026-11-06",
        "ordersCount": 10,
        "rejectedCount": 1,
        "cancelledCount": 1,
        "failedCount": 2,
        "failureRateBps": 2000
      }
    ]
  }
}
```

### 3.3 `GET /api/v1/analytics/restaurants/{restaurantId}/delivery-avg?from=YYYY-MM-DD&to=YYYY-MM-DD`
- **Auth**: JWT + `RequireRestaurantMember("restaurantId")` + RBAC `analytics:read`
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": {
    "totalDelivered": 2,
    "avgDeliveryMs": 1800000,
    "avgDeliverySec": 1800,
    "days": [
      {
        "date": "2026-11-06",
        "deliveredCount": 2,
        "avgDeliveryMs": 1800000,
        "avgDeliverySec": 1800
      }
    ]
  }
}
```

### 3.4 `GET /api/v1/analytics/restaurants/{restaurantId}/branches/days?from=YYYY-MM-DD&to=YYYY-MM-DD&branchId=1`
- **Auth**: JWT + `RequireRestaurantMember` + RBAC `analytics:read` + `RequireBranchAccess("branchId")`
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": [
    {
      "branchId": 1,
      "restaurantId": 42,
      "date": "2026-11-06",
      "currency": "EGP",
      "ordersCount": 2,
      "deliveredCount": 1,
      "rejectedCount": 0,
      "cancelledCount": 0,
      "revenueMinor": 4000,
      "avgOrderMinor": 2000,
      "avgDeliveryMs": 1800000
    }
  ]
}
```

### 3.5 `GET /api/v1/analytics/restaurants/{restaurantId}/products/top?from=YYYY-MM-DD&to=YYYY-MM-DD&branchId=1&limit=10`
- **Auth**: JWT + `RequireRestaurantMember` + RBAC `analytics:read` + `RequireBranchAccess("branchId")`
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": [
    {
      "productId": 10,
      "quantitySold": 4,
      "revenueMinor": 4000,
      "ordersCount": 2,
      "currency": "EGP"
    }
  ]
}
```

### 3.6 `GET /api/v1/analytics/platform/days?from=YYYY-MM-DD&to=YYYY-MM-DD&region=eg`
- **Auth**: JWT + `RequireAdmin` (`system_admin`)
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": [
    {
      "date": "2026-11-06",
      "region": "eg",
      "currency": "EGP",
      "ordersCount": 2,
      "deliveredCount": 1,
      "rejectedCount": 0,
      "cancelledCount": 0,
      "gmvMinor": 4000,
      "commissionMinor": 250,
      "serviceFeeMinor": 200,
      "deliveryFeeMinor": 200,
      "onlinePaymentCount": 1,
      "onlinePaymentMinor": 1500,
      "paymentFailedCount": 0,
      "avgOrderMinor": 2000,
      "avgDeliveryMs": 1800000
    }
  ]
}
```

### 3.7 `GET /api/v1/analytics/platform/active-restaurants?from=YYYY-MM-DD&to=YYYY-MM-DD`
- **Auth**: JWT + `RequireAdmin` (`system_admin`)
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": {
    "totalActiveRestaurants": 1,
    "days": [
      {
        "date": "2026-11-06",
        "activeRestaurantsCount": 1,
        "ordersCount": 2,
        "revenueMinor": 4000
      }
    ]
  }
}
```

### 3.8 `GET /api/v1/analytics/platform/failures?from=YYYY-MM-DD&to=YYYY-MM-DD&region=eg`
- **Auth**: JWT + `RequireAdmin` (`system_admin`)
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "data": {
    "totalOrders": 2,
    "totalRejected": 1,
    "totalCancelled": 0,
    "totalPaymentFailed": 0,
    "orderFailureBps": 5000,
    "days": [
      {
        "date": "2026-11-06",
        "region": "eg",
        "ordersCount": 2,
        "rejectedCount": 1,
        "cancelledCount": 0,
        "paymentFailedCount": 0,
        "orderFailureBps": 5000
      }
    ]
  }
}
```

