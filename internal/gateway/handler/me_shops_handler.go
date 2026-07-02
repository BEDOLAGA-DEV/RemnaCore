package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sort"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// UserBindingLister lists all role assignments for a user. It is satisfied by
// rbac.Repository and exists as a narrow port so the handler stays decoupled
// from the full repository interface and is easily stub-testable.
type UserBindingLister interface {
	ListBindingsForUser(ctx context.Context, userID string) ([]rbac.Binding, error)
}

// ShopNameGetter resolves a tenant's display name from its ID. The reseller
// tenants table carries no RLS, so no special context scoping is required.
// It is satisfied by the adapter wired in internal/gateway/module.go.
type ShopNameGetter interface {
	GetTenantName(ctx context.Context, id string) (string, error)
}

// shopItem is the JSON-serialisable shape of one shop membership entry.
type shopItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MeShopsHandler serves GET /api/me/shops, returning the authenticated caller's
// shop memberships as a stable, name-sorted JSON array.
type MeShopsHandler struct {
	bindings UserBindingLister
	tenants  ShopNameGetter
	log      *slog.Logger
}

// NewMeShopsHandler constructs a MeShopsHandler.
func NewMeShopsHandler(bindings UserBindingLister, tenants ShopNameGetter, log *slog.Logger) *MeShopsHandler {
	return &MeShopsHandler{bindings: bindings, tenants: tenants, log: log}
}

// MyShops handles GET /api/me/shops.
//
// Response: 200 with a JSON array of {"id","name"} objects, sorted by name
// then by ID as a tiebreaker. The array is always initialised (never null).
// Tenants that cannot be resolved (e.g. deleted) are silently skipped.
func (h *MeShopsHandler) MyShops(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	allBindings, err := h.bindings.ListBindingsForUser(r.Context(), claims.UserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	// Collect unique shop tenant IDs. A user may hold multiple roles in a single
	// shop; dedup by tenant ID so each shop appears exactly once.
	seen := make(map[string]struct{})
	var tenantIDs []string
	for _, b := range allBindings {
		if b.ScopeKind != rbac.ScopeShop || b.TenantID == nil {
			continue
		}
		if _, dup := seen[*b.TenantID]; !dup {
			seen[*b.TenantID] = struct{}{}
			tenantIDs = append(tenantIDs, *b.TenantID)
		}
	}

	items := make([]shopItem, 0, len(tenantIDs))
	for _, id := range tenantIDs {
		name, err := h.tenants.GetTenantName(r.Context(), id)
		if err != nil {
			// Tenant may have been deleted or is otherwise unresolvable.
			// Skip rather than fail the entire list; log at warn for observability.
			h.log.WarnContext(r.Context(), "me/shops: skipping unresolvable tenant",
				slog.String("tenant_id", id),
				slog.Any("err", err),
			)
			continue
		}
		items = append(items, shopItem{ID: id, Name: name})
	}

	// Stable order: sort by name, then by ID as tiebreaker.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})

	writeJSON(w, http.StatusOK, items)
}
