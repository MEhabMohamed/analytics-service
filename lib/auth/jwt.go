// Package auth implements JWT verification, HTTP authentication guards, and internal API-key checks.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/auth/jwt.ts` (`verifyAccessToken`).
//   - In Go (`github.com/golang-jwt/jwt/v5`), `jwtClaims` embeds `jwt.RegisteredClaims` (struct embedding,
//     Go's composition replacement for `jwt.JwtPayload & JWTPayload` intersection types in TypeScript).
//   - We also verify `t.Method.(*jwt.SigningMethodHMAC)` inside the key callback to prevent `alg: none`
//     or algorithm-confusion attacks.
//   - In Node/PostgreSQL (Knex), bigint IDs (e.g. restaurantId, userId, branchIds) may be serialized as
//     strings (e.g. "5") or numbers (5) depending on the caller or DB driver. flexInt64 and flexInt64Slice
//     support unmarshaling both JSON numbers and JSON strings transparently.
package auth

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"quickbite/analytics-service/lib/appcontext"
)

type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	s := strings.Trim(string(data), "\"")
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		flt, fltErr := strconv.ParseFloat(s, 64)
		if fltErr != nil {
			return fmt.Errorf("cannot parse %q as int64: %w", string(data), err)
		}
		val = int64(flt)
	}
	*f = flexInt64(val)
	return nil
}

type flexInt64Slice []int64

func (s *flexInt64Slice) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	res := make([]int64, 0, len(raw))
	for _, item := range raw {
		var fi flexInt64
		if err := json.Unmarshal(item, &fi); err != nil {
			return err
		}
		res = append(res, int64(fi))
	}
	*s = res
	return nil
}

type jwtClaims struct {
	UserID         flexInt64      `json:"userId"`
	Role           string         `json:"role"`
	Email          string         `json:"email"`
	RestaurantID   *flexInt64     `json:"restaurantId,omitempty"`
	RestaurantRole *string        `json:"restaurantRole,omitempty"`
	BranchIDs      flexInt64Slice `json:"branchIds,omitempty"`
	jwt.RegisteredClaims
}

// VerifyAccessToken validates an HS256 JWT with the shared ACCESS_SECRET and extracts Claims.
func VerifyAccessToken(tokenStr string, secret string) (*appcontext.Claims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("token string is empty")
	}

	parsed, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt parse/signature failed: %w", err)
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("jwt is invalid or expired")
	}

	c, ok := parsed.Claims.(*jwtClaims)
	if !ok {
		return nil, fmt.Errorf("claims type mismatch: expected *jwtClaims, got %T", parsed.Claims)
	}
	if c.UserID == 0 {
		return nil, fmt.Errorf("missing or zero userId in claims")
	}
	if c.Role == "" {
		return nil, fmt.Errorf("missing role in claims")
	}

	var restID *int64
	if c.RestaurantID != nil {
		id := int64(*c.RestaurantID)
		restID = &id
	}

	return &appcontext.Claims{
		UserID:         int64(c.UserID),
		Role:           c.Role,
		Email:          c.Email,
		RestaurantID:   restID,
		RestaurantRole: c.RestaurantRole,
		BranchIDs:      []int64(c.BranchIDs),
	}, nil
}
