// Package analytics holds shared module-level input and query types.
//
// Divided into 2 cohesive files in `package analytics`:
//   - types.go       : Inbound RabbitMQ event inputs + service query filter inputs
//   - projections.go : Service output row and summary structs
//
// TS vs Go Syntax Note:
//   - In TypeScript (`order-service/src/app/order/types.ts`), shared shapes are declared with
//     `export interface ReserveStockInput { ... }`.
//   - In Go, `interface` is ONLY for method sets (behavior), never for data fields!
//     Data shapes are always declared with `type Name struct { ... }`.
//   - Notice the struct tags (`json:"orderId"` or `bson:"_id"`): in TypeScript, JSON property
//     names match the TS identifier directly; in Go, struct fields must be `PascalCase` to be
//     exported, so struct tags tell `encoding/json` or `mongo-driver/bson` the exact wire/DB key name.
package analytics

// ─── Inbound Event Inputs ────────────────────────────────────────────────────

// OrderPlacedItemInput represents a single line item inside an order.placed event.
type OrderPlacedItemInput struct {
	ProductID int64 `json:"productId"`
	Quantity  int64 `json:"quantity"`
	UnitPrice int64 `json:"unitPrice"`
	LineTotal int64 `json:"lineTotal"`
}

// OnOrderPlacedInput matches order-service's order.placed payload.
type OnOrderPlacedInput struct {
	OrderID       string                 `json:"orderId"`
	Region        string                 `json:"region"`
	CountryCode   string                 `json:"countryCode"`
	RestaurantID  int64                  `json:"restaurantId"`
	BranchID      int64                  `json:"branchId"`
	CustomerID    int64                  `json:"customerId"`
	Status        string                 `json:"status"`
	PaymentMethod string                 `json:"paymentMethod"`
	Subtotal      int64                  `json:"subtotal"`
	DeliveryFee   int64                  `json:"deliveryFee"`
	ServiceFee    int64                  `json:"serviceFee"`
	Total         int64                  `json:"total"`
	Currency      string                 `json:"currency"`
	Items         []OrderPlacedItemInput `json:"items"`
	PlacedAt      string                 `json:"placedAt"`
}

// OnPaymentCompletedInput matches order-service's payment.completed payload.
type OnPaymentCompletedInput struct {
	OrderID             string `json:"orderId"`
	Region              string `json:"region"`
	RestaurantID        int64  `json:"restaurantId"`
	BranchID            int64  `json:"branchId"`
	CustomerID          int64  `json:"customerId"`
	Provider            string `json:"provider"`
	ProviderReferenceID string `json:"providerReferenceId"`
	Amount              int64  `json:"amount"`
	Currency            string `json:"currency"`
	Method              string `json:"method"`
	CompletedAt         string `json:"completedAt"`
}

// OnPaymentFailedInput matches order-service's payment.failed payload.
type OnPaymentFailedInput struct {
	OrderID             string `json:"orderId"`
	Region              string `json:"region"`
	RestaurantID        int64  `json:"restaurantId"`
	BranchID            int64  `json:"branchId"`
	CustomerID          int64  `json:"customerId"`
	Provider            string `json:"provider"`
	ProviderReferenceID string `json:"providerReferenceId"`
	Amount              int64  `json:"amount"`
	Currency            string `json:"currency"`
	Method              string `json:"method"`
	FailedAt            string `json:"failedAt"`
}

// OnOrderDeliveredInput matches order-service's order.delivered payload.
type OnOrderDeliveredInput struct {
	OrderID         string `json:"orderId"`
	Region          string `json:"region"`
	CountryCode     string `json:"countryCode"`
	RestaurantID    int64  `json:"restaurantId"`
	BranchID        int64  `json:"branchId"`
	CustomerID      int64  `json:"customerId"`
	DeliveryAgentID int64  `json:"deliveryAgentId"`
	Total           int64  `json:"total"`
	Subtotal        int64  `json:"subtotal"`
	DeliveryFee     int64  `json:"deliveryFee"`
	Commission      int64  `json:"commission"`
	Currency        string `json:"currency"`
	PaymentMethod   string `json:"paymentMethod"`
	PlacedAt        string `json:"placedAt"`
	DeliveredAt     string `json:"deliveredAt"`
}

// OnOrderTransitionInput matches order-service's order.rejected / order.cancelled payload.
type OnOrderTransitionInput struct {
	OrderID      string  `json:"orderId"`
	Region       string  `json:"region"`
	CountryCode  string  `json:"countryCode"`
	RestaurantID int64   `json:"restaurantId"`
	BranchID     int64   `json:"branchId"`
	CustomerID   int64   `json:"customerId"`
	Status       string  `json:"status"`
	Currency     string  `json:"currency"`
	Reason       *string `json:"reason"`
	Actor        string  `json:"actor"`
	PlacedAt     string  `json:"placedAt"`
	OccurredAt   string  `json:"occurredAt"`
}

// OnRBACPermissionsChangedInput matches core-service's rbac.permissions_changed payload.
type OnRBACPermissionsChangedInput struct {
	Role string `json:"role"`
}

// ─── Service Query Inputs ────────────────────────────────────────────────────

// DateRangeQuery holds validated date bounds ("YYYY-MM-DD").
type DateRangeQuery struct {
	RestaurantID int64
	BranchID     int64
	Region       string
	From         string
	To           string
	Limit        int64
}
