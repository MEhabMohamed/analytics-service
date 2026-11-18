// Package coreclient implements the synchronous HTTP client to core-service.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/core-client/types.ts` (`CoreEnvelope<T>`, `RolePermissionsResponse`).
//   - Uses Go 1.18+ generics (`type Envelope[T any] struct`) to wrap typed `Data T` payloads cleanly.
package coreclient

import "encoding/json"

// Envelope matches core-service's {"success": true, "data": ...} wire format.
type Envelope[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
}

// RolePermissionItem represents a single permission entry in RolePermissionsResponse.
// Supports both string elements ("analytics:read") and object elements ({"permission": "analytics:read"}).
type RolePermissionItem struct {
	Permission string `json:"permission"`
}

// UnmarshalJSON decodes either a JSON string or a {"permission": "..."} object.
func (r *RolePermissionItem) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Permission = s
		return nil
	}
	var obj struct {
		Permission string `json:"permission"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	r.Permission = obj.Permission
	return nil
}

// RolePermissionsResponse matches GET /api/internal/rbac/permissions?role=...
type RolePermissionsResponse struct {
	Role        string               `json:"role"`
	Permissions []RolePermissionItem `json:"permissions"`
}

