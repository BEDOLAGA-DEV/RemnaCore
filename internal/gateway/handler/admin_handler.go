package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

// AdminHandler exposes HTTP endpoints for the admin panel.
//
// All billing data access is read-only (list subscriptions, list invoices),
// so the handler depends on Reader interfaces rather than full Repository
// interfaces.
type AdminHandler struct {
	identitySvc *identity.Service
	subs        billing.SubscriptionReader
	invoices    billing.InvoiceReader
	txRunner    txmanager.Runner
}

// NewAdminHandler creates an AdminHandler backed by the given services.
func NewAdminHandler(
	identitySvc *identity.Service,
	subs billing.SubscriptionReader,
	invoices billing.InvoiceReader,
	txRunner txmanager.Runner,
) *AdminHandler {
	return &AdminHandler{
		identitySvc: identitySvc,
		subs:        subs,
		invoices:    invoices,
		txRunner:    txRunner,
	}
}

// parsePagination extracts limit and offset from query parameters.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultPageLimit
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

// ListUsers handles GET /api/admin/users -- list all users (paginated).
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	users, err := h.identitySvc.ListUsers(r.Context(), limit, offset)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, users)
}

// GetUser handles GET /api/admin/users/{userID} -- get a single user detail.
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("user ID is required"))
		return
	}

	user, err := h.identitySvc.GetMe(r.Context(), userID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

// ListSubscriptions handles GET /api/admin/subscriptions -- list all
// subscriptions (paginated).
func (h *AdminHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	// Wrap the read in a tx so the platform-admin sentinel GUC set by ShopResolver
	// is applied; on the bare pool the GUC is never set and FORCE-RLS (post-042)
	// would return zero rows. This route is platform-only gated.
	var subs []*aggregate.Subscription
	err := h.txRunner.RunInTx(r.Context(), func(txCtx context.Context) error {
		var err error
		subs, err = h.subs.GetAll(txCtx, limit, offset)
		return err
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, subs)
}

// ListInvoices handles GET /api/admin/invoices -- list all invoices (paginated).
func (h *AdminHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	// Wrap the read in a tx so the platform-admin sentinel GUC set by ShopResolver
	// is applied; on the bare pool the GUC is never set and FORCE-RLS (post-042)
	// would return zero rows. This route is platform-only gated.
	var invoices []*aggregate.Invoice
	err := h.txRunner.RunInTx(r.Context(), func(txCtx context.Context) error {
		var err error
		invoices, err = h.invoices.GetAll(txCtx, limit, offset)
		return err
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invoices)
}
