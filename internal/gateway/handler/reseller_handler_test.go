package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
)

func TestResellerCommissions_NoActiveTenant_Forbidden(t *testing.T) {
	h := NewResellerHandler(&reseller.ResellerService{})

	req := httptest.NewRequest(http.MethodGet, "/api/reseller/commissions", nil)
	rec := httptest.NewRecorder()

	h.Commissions(rec, req)

	// VERIFIED (correction #21): apierror.Error serializes Details via
	// `json:"details,omitempty"` (pkg/apierror/error.go) and writeAPIError
	// encodes the whole struct (handler/response.go), so the Details string IS
	// in the body. We still assert on the STABLE COMMON.FORBIDDEN code + 403
	// status rather than the human-readable Details text, which is an
	// implementation detail that may be reworded.
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "COMMON.FORBIDDEN")
}
