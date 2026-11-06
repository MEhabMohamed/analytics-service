// Package auth implements JWT verification, HTTP authentication guards, and internal API-key checks.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/auth/jwt.ts` (`verifyAccessToken`).
//   - In Go (`github.com/golang-jwt/jwt/v5`), `jwtClaims` embeds `jwt.RegisteredClaims` (struct embedding,
//     Go's composition replacement for `jwt.JwtPayload & JWTPayload` intersection types in TypeScript).
//   - We also verify `t.Method.(*jwt.SigningMethodHMAC)` inside the key callback to prevent `alg: none`
//     or algorithm-confusion attacks.
package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"quickbite/analytics-service/lib/appcontext"
	apperr "quickbite/analytics-service/lib/errors"
)

type jwtClaims struct {
	UserID         int64   `json:"userId"`
	Role           string  `json:"role"`
	Email          string  `json:"email"`
	RestaurantID   *int64  `json:"restaurantId,omitempty"`
	RestaurantRole *string `json:"restaurantRole,omitempty"`
	BranchIDs      []int64 `json:"branchIds,omitempty"`
	jwt.RegisteredClaims
}

// VerifyAccessToken validates an HS256 JWT with the shared ACCESS_SECRET and extracts Claims.
func VerifyAccessToken(tokenStr string, secret string) (*appcontext.Claims, error) {
	if tokenStr == "" {
		return nil, apperr.ErrUnauthenticated
	}

	parsed, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, apperr.ErrUnauthenticated
	}

	c, ok := parsed.Claims.(*jwtClaims)
	if !ok || c.UserID == 0 || c.Role == "" {
		return nil, apperr.ErrUnauthenticated
	}

	return &appcontext.Claims{
		UserID:         c.UserID,
		Role:           c.Role,
		Email:          c.Email,
		RestaurantID:   c.RestaurantID,
		RestaurantRole: c.RestaurantRole,
		BranchIDs:      c.BranchIDs,
	}, nil
}

