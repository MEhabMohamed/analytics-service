// Package coreclient implements the synchronous HTTP client to core-service.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/core-client/types.ts` (`CoreEnvelope<T>`, `RolePermissionsResponse`).
//   - Uses Go 1.18+ generics (`type Envelope[T any] struct`) to wrap typed `Data T` payloads cleanly.
package coreclient

// Envelope matches core-service's {"success": true, "data": ...} wire format.
type Envelope[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

// RolePermissionsResponse matches GET /api/internal/rbac/permissions?role=...
type RolePermissionsResponse struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

