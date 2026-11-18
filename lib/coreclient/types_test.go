package coreclient

import (
	"encoding/json"
	"testing"
)

func TestRolePermissionsResponse_UnmarshalBothShapes(t *testing.T) {
	t.Run("string array shape", func(t *testing.T) {
		raw := `{"success":true,"data":{"role":"owner","permissions":["analytics:read","orders:read"]}}`
		var env Envelope[RolePermissionsResponse]
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			t.Fatalf("unmarshal string array shape: %v", err)
		}
		if env.Data.Role != "owner" {
			t.Fatalf("expected role owner, got %q", env.Data.Role)
		}
		if len(env.Data.Permissions) != 2 || env.Data.Permissions[0].Permission != "analytics:read" || env.Data.Permissions[1].Permission != "orders:read" {
			t.Fatalf("unexpected permissions: %+v", env.Data.Permissions)
		}
	})

	t.Run("object array shape", func(t *testing.T) {
		raw := `{"success":true,"data":{"role":"branch_manager","permissions":[{"permission":"analytics:read"},{"permission":"branches:read"}]}}`
		var env Envelope[RolePermissionsResponse]
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			t.Fatalf("unmarshal object array shape: %v", err)
		}
		if env.Data.Role != "branch_manager" {
			t.Fatalf("expected role branch_manager, got %q", env.Data.Role)
		}
		if len(env.Data.Permissions) != 2 || env.Data.Permissions[0].Permission != "analytics:read" || env.Data.Permissions[1].Permission != "branches:read" {
			t.Fatalf("unexpected permissions: %+v", env.Data.Permissions)
		}
	})

	t.Run("invalid element fails", func(t *testing.T) {
		raw := `{"role":"owner","permissions":[123]}`
		var resp RolePermissionsResponse
		if err := json.Unmarshal([]byte(raw), &resp); err == nil {
			t.Fatalf("expected error for numeric permission element")
		}
	})
}
