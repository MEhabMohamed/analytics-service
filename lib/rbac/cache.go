// Package rbac implements the in-process read-through permission cache and HTTP RBAC middleware.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/rbac/permission-cache.service.ts` (`PermissionCacheService`).
//   - In Node.js, a plain `Map<string, ...>` is safe because JS executes on a single-threaded event loop.
//   - In Go, HTTP requests and AMQP consumers run concurrently across multiple goroutines/OS threads,
//     so `entries map[string]cacheEntry` is guarded by a read-write mutex (`sync.RWMutex`):
//     `c.mu.RLock()` allows concurrent cache hits, while `c.mu.Lock()` serializes cache writes/invalidations.
package rbac

import (
	"context"
	"sync"
	"time"
)

// PermissionFetcher abstracts coreclient.Client for unit testability.
type PermissionFetcher interface {
	GetPermissionsByRole(ctx context.Context, role string) ([]string, error)
}

type cacheEntry struct {
	perms    map[string]struct{}
	raw      []string
	cachedAt time.Time
}

// Cache is a thread-safe in-process read-through cache for role permissions.
// Mirrors order-service/src/lib/rbac/permission-cache.service.ts.
type Cache struct {
	fetcher PermissionFetcher
	ttl     time.Duration
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// NewCache creates a new RBAC permission Cache with the given TTL.
func NewCache(fetcher PermissionFetcher, ttl time.Duration) *Cache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Cache{
		fetcher: fetcher,
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

// HasPermission checks whether the given role has perm (e.g. "analytics:read"), fetching from core-service on cache miss/expiry.
func (c *Cache) HasPermission(ctx context.Context, role string, perm string) (bool, error) {
	if role == "" {
		return false, nil
	}

	c.mu.RLock()
	entry, ok := c.entries[role]
	c.mu.RUnlock()

	if ok && time.Since(entry.cachedAt) < c.ttl {
		_, allowed := entry.perms[perm]
		return allowed, nil
	}

	perms, err := c.fetcher.GetPermissionsByRole(ctx, role)
	if err != nil {
		// Serve stale cache up to 1 hour if core-service is temporarily unreachable
		if ok && time.Since(entry.cachedAt) < time.Hour {
			_, allowed := entry.perms[perm]
			return allowed, nil
		}
		return false, err
	}

	permSet := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		permSet[p] = struct{}{}
	}

	c.mu.Lock()
	c.entries[role] = cacheEntry{
		perms:    permSet,
		raw:      perms,
		cachedAt: time.Now(),
	}
	c.mu.Unlock()

	_, allowed := permSet[perm]
	return allowed, nil
}

// Invalidate removes a specific role from the cache, or clears all roles if role == "".
func (c *Cache) Invalidate(role string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if role == "" {
		c.entries = make(map[string]cacheEntry)
		return
	}
	delete(c.entries, role)
}

