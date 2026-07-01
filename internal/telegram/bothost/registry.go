package bothost

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

var (
	// ErrUnknownOp is returned by Call for an op that was never registered.
	ErrUnknownOp = errors.New("bothost: unknown op")
	// ErrPermissionDenied is returned when the granted set lacks the op's
	// required permission.
	ErrPermissionDenied = errors.New("bothost: op permission denied")
)

// PermSet is the set of permission scopes a plugin has been granted.
type PermSet map[plugin.PermissionScope]struct{}

// NewPermSet builds a PermSet from the given scopes.
func NewPermSet(perms ...plugin.PermissionScope) PermSet {
	s := make(PermSet, len(perms))
	for _, p := range perms {
		s[p] = struct{}{}
	}
	return s
}

// Has reports whether the scope is granted.
func (p PermSet) Has(s plugin.PermissionScope) bool {
	_, ok := p[s]
	return ok
}

// registeredOp pairs a handler with the permission it requires ("" = none).
type registeredOp struct {
	perm plugin.PermissionScope
	fn   OpHandler
}

// Registry maps op names to permission-gated handlers. Safe for concurrent use.
type Registry struct {
	mu  sync.RWMutex
	ops map[string]registeredOp
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{ops: make(map[string]registeredOp)}
}

// Register binds an op name to a handler requiring perm (empty = no permission
// required). Re-registering an op replaces it.
func (r *Registry) Register(op string, perm plugin.PermissionScope, fn OpHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ops[op] = registeredOp{perm: perm, fn: fn}
}

// Call runs op with args under oc, after checking that granted holds the op's
// required permission. Unknown op → ErrUnknownOp; missing permission →
// ErrPermissionDenied. Tenant scoping (RunInTx) is applied by individual ops,
// not here, because sends are not transactional work.
func (r *Registry) Call(ctx context.Context, oc *OpContext, granted PermSet, op string, args json.RawMessage) (json.RawMessage, error) {
	r.mu.RLock()
	ro, ok := r.ops[op]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownOp
	}
	if ro.perm != "" && !granted.Has(ro.perm) {
		return nil, ErrPermissionDenied
	}
	return ro.fn(ctx, oc, args)
}

// Bind returns a Host that forwards every Call to this registry with the bound
// OpContext and granted permission set — one bound Host per update dispatch.
func (r *Registry) Bind(oc *OpContext, granted PermSet) Host {
	return &boundHost{registry: r, oc: oc, granted: granted}
}

type boundHost struct {
	registry *Registry
	oc       *OpContext
	granted  PermSet
}

func (h *boundHost) Call(ctx context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	return h.registry.Call(ctx, h.oc, h.granted, op, args)
}
