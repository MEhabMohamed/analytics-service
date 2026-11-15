package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"quickbite/analytics-service/lib/auth"
)

func TestVerifyAccessToken_StringAndNumberIDs(t *testing.T) {
	const secret = "test-access-secret"

	tests := []struct {
		name         string
		claims       jwt.MapClaims
		wantUserID   int64
		wantRestID   *int64
		wantRole     string
		wantBranches []int64
		wantErr      bool
	}{
		{
			name: "numeric IDs",
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
			wantBranches: []int64{1, 2},
		},
		{
			name: "stringified IDs (Postgres bigint style)",
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
			wantBranches: []int64{1, 2},
		},
		{
			name: "system_admin without restaurantId",
			claims: jwt.MapClaims{
				"userId": "99",
				"role":   "system_admin",
				"email":  "admin@test.com",
				"exp":    time.Now().Add(time.Hour).Unix(),
			},
			wantUserID:   99,
			wantRestID:   nil,
			wantRole:     "system_admin",
			wantBranches: []int64{},
		},
		{
			name: "expired token",
			claims: jwt.MapClaims{
				"userId": 10,
				"role":   "system_admin",
				"email":  "admin@test.com",
				"exp":    time.Now().Add(-time.Minute).Unix(),
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := jwt.NewWithClaims(jwt.SigningMethodHS256, tc.claims)
			tokenStr, err := tok.SignedString([]byte(secret))
			if err != nil {
				t.Fatalf("sign token: %v", err)
			}

			got, err := auth.VerifyAccessToken(tokenStr, secret)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.UserID != tc.wantUserID {
				t.Errorf("userId = %d, want %d", got.UserID, tc.wantUserID)
			}
			if got.Role != tc.wantRole {
				t.Errorf("role = %q, want %q", got.Role, tc.wantRole)
			}
			if tc.wantRestID == nil {
				if got.RestaurantID != nil {
					t.Errorf("restaurantId = %v, want nil", *got.RestaurantID)
				}
			} else if got.RestaurantID == nil || *got.RestaurantID != *tc.wantRestID {
				t.Errorf("restaurantId = %v, want %d", got.RestaurantID, *tc.wantRestID)
			}
			if len(got.BranchIDs) != len(tc.wantBranches) {
				t.Fatalf("branchIds len = %d, want %d", len(got.BranchIDs), len(tc.wantBranches))
			}
			for i, b := range got.BranchIDs {
				if b != tc.wantBranches[i] {
					t.Errorf("branchIds[%d] = %d, want %d", i, b, tc.wantBranches[i])
				}
			}
		})
	}
}

func ptrInt64(v int64) *int64 { return &v }

