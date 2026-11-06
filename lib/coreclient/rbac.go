// Package coreclient provides the RBAC permission lookup against core-service.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/core-client/rbac.client.ts` (`getPermissionsByRole`).
//   - Uses `url.QueryEscape(role)` (Go's stdlib equivalent of JS `encodeURIComponent(role)`).
package coreclient

import (
	"context"
	"fmt"
	"net/url"
)

// GetPermissionsByRole fetches the permission list for a restaurant role from core-service.
func (c *Client) GetPermissionsByRole(ctx context.Context, role string) ([]string, error) {
	var env Envelope[RolePermissionsResponse]
	path := fmt.Sprintf("/api/internal/rbac/permissions?role=%s", url.QueryEscape(role))
	if err := c.getJSON(ctx, path, &env); err != nil {
		return nil, err
	}
	if env.Data.Permissions == nil {
		return []string{}, nil
	}
	return env.Data.Permissions, nil
}

