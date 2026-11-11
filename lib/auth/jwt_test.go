package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifyAccessToken_StringAndNumberIDs(t *testing.T) {
	secret := "test-secret-key-12345678"

	tests := []struct {
		name         string
		claims       jwt.MapClaims
		wantUserID   int64
		wantRestID   *int64
		wantRole     string
		wantRestRole *string
		wantBranches []int64
		wantErr      bool
	}{
		{
			name: "numeric IDs as numbers",
			claims: jwt.MapClaims{
				"userId":         10,
				"role":           "restaurant_user",
				"email":          "owner@test.com",
				"restaurantId":   5,
				"restaurantRole": "owner",
				"branchIds":      []any{1, 2},
				"exp":            time.Now().Add(time.Hour).Unix(),
			},
			wantUserID:   10,
			wantRestID:   ptrInt64(5),
			wantRole:     "restaurant_user",
			wantRestRole: ptrString("owner"),
			wantBranches: []int64{1, 2},
			wantErr:      false,
		},
		{
			name: "stringified IDs (Knex / Postgres bigint style)",
			claims: jwt.MapClaims{
				"userId":         "10",
				"role":           "restaurant_user",
				"email":          "owner@test.com",
				"restaurantId":   "5",
				"restaurantRole": "owner",
				"branchIds":      []any{"1", "2"},
				"exp":            time.Now().Add(time.Hour).Unix(),
			},
			wantUserID:   10,
			wantRestID:   ptrInt64(5),
			wantRole:     "restaurant_user",
			wantRestRole: ptrString("owner"),
			wantBranches: []int64{1, 2},
			wantErr:      false,
		},
		{
			name: "system_admin with no restaurantId or branchIds",
			claims: jwt.MapClaims{
				"userId": "99",
				"role":   "system_admin",
				"email":  "admin@test.com",
				"exp":    time.Now().Add(time.Hour).Unix(),
			},
			wantUserID:   99,
			wantRestID:   nil,
			wantRole:     "system_admin",
			wantRestRole: nil,
			wantBranches: []int64{},
			wantErr:      false,
		},
		{
			name: "restaurantId is null",
			claims: jwt.MapClaims{
				"userId":       10,
				"role":         "customer",
				"email":        "cust@test.com",
				"restaurantId": nil,
				"exp":          time.Now().Add(time.Hour).Unix(),
			},
			wantUserID:   10,
			wantRestID:   nil,
			wantRole:     "customer",
			wantRestRole: nil,
			wantBranches: []int64{},
			wantErr:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, tc.claims)
			tokenStr, err := token.SignedString([]byte(secret))
			if err != nil {
				t.Fatalf("failed to sign token: %v", err)
			}

			claims, err := VerifyAccessToken(tokenStr, secret)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if claims.UserID != tc.wantUserID {
				t.Errorf("userId = %d, want %d", claims.UserID, tc.wantUserID)
			}
			if tc.wantRestID == nil {
				if claims.RestaurantID != nil {
					t.Errorf("restaurantId = %v, want nil", *claims.RestaurantID)
				}
			} else {
				if claims.RestaurantID == nil || *claims.RestaurantID != *tc.wantRestID {
					t.Errorf("restaurantId = %v, want %v", claims.RestaurantID, *tc.wantRestID)
				}
			}
			if claims.Role != tc.wantRole {
				t.Errorf("role = %q, want %q", claims.Role, tc.wantRole)
			}
			if tc.wantRestRole == nil {
				if claims.RestaurantRole != nil {
					t.Errorf("restaurantRole = %v, want nil", *claims.RestaurantRole)
				}
			} else {
				if claims.RestaurantRole == nil || *claims.RestaurantRole != *tc.wantRestRole {
					t.Errorf("restaurantRole = %v, want %v", claims.RestaurantRole, *tc.wantRestRole)
				}
			}
			if len(claims.BranchIDs) != len(tc.wantBranches) {
				t.Fatalf("branchIds len = %d, want %d", len(claims.BranchIDs), len(tc.wantBranches))
			}
			for i, b := range claims.BranchIDs {
				if b != tc.wantBranches[i] {
					t.Errorf("branchIds[%d] = %d, want %d", i, b, tc.wantBranches[i])
				}
			}
		})
	}
}

func ptrInt64(v int64) *int64    { return &v }
func ptrString(v string) *string { return &v }

