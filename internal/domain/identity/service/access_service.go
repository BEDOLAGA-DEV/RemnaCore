package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// EffectiveAccess is a user's resolved authorization for an active tenant.
type EffectiveAccess struct {
	UserID          string
	IsPlatformAdmin bool
	Permissions     map[rbac.Permission]struct{}
	AllowedTenants  map[string]struct{}
}

// AccessCacheTTL bounds how long a resolved EffectiveAccess is reused. A revoked
// grant can remain effective for up to this long per node; emergency revoke must
// also terminate sessions/refresh tokens (see the design spec, §6). Exported so
// fx wiring (Task 11) can pass it to NewAccessService.
const AccessCacheTTL = 60 * time.Second

type cacheEntry struct {
	access    EffectiveAccess
	expiresAt time.Time
}

// AccessService resolves and caches effective permissions. The cache is a small
// purpose-built typed store (not the proxy LRUCache, which holds []byte bodies
// and offers no per-key invalidation) keyed by userID + active tenant.
type AccessService struct {
	repo rbac.Repository
	now  func() time.Time
	ttl  time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry // key: userID + "|" + tenantKey
}

// NewAccessService constructs an AccessService. now and ttl are injected for
// testability; production wiring passes time.Now and AccessCacheTTL.
func NewAccessService(repo rbac.Repository, now func() time.Time, ttl time.Duration) *AccessService {
	return &AccessService{repo: repo, now: now, ttl: ttl, cache: map[string]cacheEntry{}}
}

func cacheKey(userID string, tenantID *string) string {
	if tenantID == nil {
		return userID + "|"
	}
	return userID + "|" + *tenantID
}

// Resolve returns the EffectiveAccess for userID under the given active tenant
// (nil = global only). Results are cached for ttl and keyed per active tenant.
func (s *AccessService) Resolve(ctx context.Context, userID string, tenantID *string) (EffectiveAccess, error) {
	key := cacheKey(userID, tenantID)
	now := s.now()

	s.mu.RLock()
	if e, ok := s.cache[key]; ok && now.Before(e.expiresAt) {
		s.mu.RUnlock()
		return e.access, nil
	}
	// NOTE: releasing the read lock before the repo call means a concurrent
	// goroutine resolving the same key may also miss the cache and make its own
	// repo call. This is benign and idempotent; single-flight is not warranted
	// for Phase A given the low expected concurrency and cheap repo reads.
	s.mu.RUnlock()

	bindings, err := s.repo.ListBindingsForUser(ctx, userID)
	if err != nil {
		return EffectiveAccess{}, err // fail closed: caller never grants on error
	}

	acc := EffectiveAccess{
		UserID:         userID,
		Permissions:    map[rbac.Permission]struct{}{},
		AllowedTenants: map[string]struct{}{},
	}
	relevantRoleIDs := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.TenantID != nil {
			acc.AllowedTenants[*b.TenantID] = struct{}{}
		}
		if b.ScopeKind == rbac.ScopeGlobal && b.RoleKey == rbac.RolePlatformAdmin {
			acc.IsPlatformAdmin = true
		}
		// A binding contributes permissions only if it is global, or scoped to
		// the active tenant.
		if b.TenantID == nil || (tenantID != nil && *b.TenantID == *tenantID) {
			relevantRoleIDs = append(relevantRoleIDs, b.RoleID)
		}
	}
	if len(relevantRoleIDs) > 0 {
		permsByRole, err := s.repo.PermissionsForRoles(ctx, relevantRoleIDs)
		if err != nil {
			return EffectiveAccess{}, err
		}
		for _, ps := range permsByRole {
			for _, p := range ps {
				acc.Permissions[p] = struct{}{}
			}
		}
	}

	s.mu.Lock()
	s.cache[key] = cacheEntry{access: acc, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()
	return acc, nil
}

// Can reports whether the access grants permission p (platform admin = allow-all).
func (s *AccessService) Can(acc EffectiveAccess, p rbac.Permission) bool {
	if acc.IsPlatformAdmin {
		return true
	}
	_, ok := acc.Permissions[p]
	return ok
}

// Invalidate drops all cached entries for a user (call on assignment grant/revoke).
func (s *AccessService) Invalidate(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.cache {
		if strings.HasPrefix(k, userID+"|") {
			delete(s.cache, k)
		}
	}
}

// Flush drops the entire cache (call on any role / role_permission mutation,
// which has no userID reverse index).
func (s *AccessService) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = map[string]cacheEntry{}
}
