package rbac_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quickbite/analytics-service/lib/rbac"
)

type fakeFetcher struct {
	perms map[string][]string
	calls int
	err   error
}

func (f *fakeFetcher) GetPermissionsByRole(_ context.Context, role string) ([]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.perms[role], nil
}

func TestRBACCache_ReadThroughAndInvalidation(t *testing.T) {
	ctx := context.Background()
	fetcher := &fakeFetcher{
		perms: map[string][]string{
			"owner": {"analytics:read", "orders:read"},
			"staff": {"orders:read"},
		},
	}

	cache := rbac.NewCache(fetcher, 5*time.Minute)

	// 1. First lookup hits fetcher
	allowed, err := cache.HasPermission(ctx, "owner", "analytics:read")
	if err != nil || !allowed {
		t.Fatalf("expected owner to have analytics:read, allowed=%v err=%v", allowed, err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("expected 1 fetcher call, got %d", fetcher.calls)
	}

	// 2. Second lookup is served from cache
	allowed, err = cache.HasPermission(ctx, "owner", "analytics:read")
	if err != nil || !allowed {
		t.Fatalf("expected cached owner permission, allowed=%v err=%v", allowed, err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("expected cached hit (still 1 call), got %d", fetcher.calls)
	}

	// 3. Staff does not have analytics:read
	allowed, err = cache.HasPermission(ctx, "staff", "analytics:read")
	if err != nil || allowed {
		t.Fatalf("expected staff to be denied analytics:read, allowed=%v err=%v", allowed, err)
	}

	// 4. Invalidate("owner") forces refresh on next check
	cache.Invalidate("owner")
	fetcher.perms["owner"] = []string{"orders:read"}

	allowed, err = cache.HasPermission(ctx, "owner", "analytics:read")
	if err != nil || allowed {
		t.Fatalf("expected invalidated owner to lose analytics:read, allowed=%v err=%v", allowed, err)
	}

	// 5. Stale fallback if fetcher errors within 1h
	fetcher.err = errors.New("core-service down")
	allowed, err = cache.HasPermission(ctx, "owner", "orders:read")
	if err != nil || !allowed {
		t.Fatalf("expected stale cache fallback on error, allowed=%v err=%v", allowed, err)
	}
}
