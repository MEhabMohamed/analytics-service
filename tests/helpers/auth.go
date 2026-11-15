package helpers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"quickbite/analytics-service/app/analytics"
)

// MintTestJWT signs an HS256 JWT token for integration tests.
func MintTestJWT(
	t *testing.T,
	secret string,
	userID int64,
	role string,
	restID *int64,
	restRole *string,
	branchIDs []int64,
) string {
	t.Helper()

	claims := jwt.MapClaims{
		"userId": userID,
		"role":   role,
		"email":  "test@quickbite.test",
		"exp":    time.Now().Add(time.Hour).Unix(),
	}
	if restID != nil {
		claims["restaurantId"] = *restID
	}
	if restRole != nil {
		claims["restaurantRole"] = *restRole
	}
	if branchIDs != nil {
		claims["branchIds"] = branchIDs
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

// NewTestCoreServer starts a local HTTP test server mimicking core-service's
// internal RBAC endpoint (/api/internal/rbac/permissions?role=...) so
// coreclient.Client and rbac.Cache execute over real HTTP without mocks.
func NewTestCoreServer(t *testing.T, internalAPIKey string) *httptest.Server {
	t.Helper()

	rolePerms := map[string][]string{
		"owner":          {analytics.PermAnalyticsRead},
		"branch_manager": {analytics.PermAnalyticsRead},
		"staff":          {"orders:read"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != internalAPIKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api/internal/rbac/permissions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		role := r.URL.Query().Get("role")
		perms := rolePerms[role]
		items := make([]map[string]string, 0, len(perms))
		for _, p := range perms {
			items = append(items, map[string]string{"permission": p})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"permissions": items,
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func PtrInt64(v int64) *int64    { return &v }
func PtrString(v string) *string { return &v }

