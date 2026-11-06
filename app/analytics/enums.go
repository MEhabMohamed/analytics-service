// Package analytics holds the shared module types, operational errors, and enum constants
// imported across subpackages (`service`, `controller`, `dto`, `repository`, `eventhandlers`).
//
// TS vs Go Syntax Note:
//   - In TypeScript (`order-service/src/app/order/enums.ts`), enums use `export enum OrderStatus { ... }`
//     or `export const EVENT_TYPES = { ... } as const`.
//   - Go does not have an `enum` keyword. Instead, Go groups compile-time constants inside
//     a `const ( ... )` block. Uppercase first letters (`PermAnalyticsRead`) export them
//     outside the package (replacing TypeScript's `export` keyword).
package analytics

// RBAC permission constants
const (
	PermAnalyticsRead = "analytics:read"
)

// Inbound event type constants
const (
	EventOrderPlaced            = "order.placed"
	EventOrderDelivered         = "order.delivered"
	EventOrderRejected          = "order.rejected"
	EventOrderCancelled         = "order.cancelled"
	EventPaymentCompleted       = "payment.completed"
	EventPaymentFailed          = "payment.failed"
	EventRBACPermissionsChanged = "rbac.permissions_changed"
)

// MongoDB collection names
const (
	CollRestaurantDay = "agg_restaurant_day"
	CollBranchDay     = "agg_branch_day"
	CollProductDay    = "agg_product_day"
	CollPlatformDay   = "agg_platform_day"
	CollEventIDs      = "event_ids"
)

