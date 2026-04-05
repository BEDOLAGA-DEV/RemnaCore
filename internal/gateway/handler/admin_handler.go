package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

// AdminHandler exposes HTTP endpoints for the admin panel.
type AdminHandler struct {
	identitySvc *identity.Service
	subs        billing.SubscriptionRepository
	invoices    billing.InvoiceRepository
}

// NewAdminHandler creates an AdminHandler backed by the given services.
func NewAdminHandler(
	identitySvc *identity.Service,
	subs billing.SubscriptionRepository,
	invoices billing.InvoiceRepository,
) *AdminHandler {
	return &AdminHandler{
		identitySvc: identitySvc,
		subs:        subs,
		invoices:    invoices,
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
	subs, err := h.subs.GetAll(r.Context(), limit, offset)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, subs)
}

// ListInvoices handles GET /api/admin/invoices -- list all invoices (paginated).
func (h *AdminHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	invoices, err := h.invoices.GetAll(r.Context(), limit, offset)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invoices)
}
