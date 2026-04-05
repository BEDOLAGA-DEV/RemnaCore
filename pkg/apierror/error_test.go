package apierror

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	err := New("TEST.CODE", "test message", http.StatusBadRequest)

	assert.Equal(t, "TEST.CODE", err.Code)
	assert.Equal(t, "test message", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.HTTPStatus)
	assert.Nil(t, err.Details)
}

func TestError_ErrorString(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		want    string
	}{
		{
			name:    "identity error",
			code:    "IDENTITY.EMAIL_TAKEN",
			message: "email already registered",
			want:    "IDENTITY.EMAIL_TAKEN: email already registered",
		},
		{
			name:    "common error",
			code:    "COMMON.INTERNAL",
			message: "internal server error",
			want:    "COMMON.INTERNAL: internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.message, http.StatusInternalServerError)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestError_ImplementsErrorInterface(t *testing.T) {
	var err error = New("TEST.CODE", "test", http.StatusOK)
	require.NotNil(t, err)
	assert.Equal(t, "TEST.CODE: test", err.Error())
}

func TestWithDetails_CreatesNewCopy(t *testing.T) {
	original := New("TEST.CODE", "original message", http.StatusBadRequest)
	details := map[string]string{"field": "email"}

	withDetails := original.WithDetails(details)

	// The copy must carry the details.
	assert.Equal(t, details, withDetails.Details)

	// The original must remain unmodified.
	assert.Nil(t, original.Details)

	// Code, message, and status must be preserved in the copy.
	assert.Equal(t, original.Code, withDetails.Code)
	assert.Equal(t, original.Message, withDetails.Message)
	assert.Equal(t, original.HTTPStatus, withDetails.HTTPStatus)
}

func TestWithDetails_DifferentDetailTypes(t *testing.T) {
	base := New("TEST.CODE", "test", http.StatusBadRequest)

	tests := []struct {
		name    string
		details any
	}{
		{
			name:    "string details",
			details: "email is required",
		},
		{
			name:    "map details",
			details: map[string]string{"field": "email"},
		},
		{
			name:    "slice details",
			details: []string{"field1", "field2"},
		},
		{
			name:    "nil details",
			details: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.WithDetails(tt.details)
			assert.Equal(t, tt.details, result.Details)
			assert.Nil(t, base.Details, "original must not be mutated")
		})
	}
}

func TestCodeConstants_HaveValidHTTPStatus(t *testing.T) {
	// Spot-check a representative set of code constants to verify they have
	// sensible HTTP status codes.
	tests := []struct {
		name       string
		err        *Error
		wantStatus int
		wantCode   string
	}{
		{"identity email taken", IdentityEmailTaken, http.StatusConflict, "IDENTITY.EMAIL_TAKEN"},
		{"identity invalid creds", IdentityInvalidCreds, http.StatusUnauthorized, "IDENTITY.INVALID_CREDENTIALS"},
		{"identity not found", IdentityNotFound, http.StatusNotFound, "IDENTITY.NOT_FOUND"},
		{"identity password short", IdentityPasswordTooShort, http.StatusUnprocessableEntity, "IDENTITY.PASSWORD_TOO_SHORT"},
		{"billing plan not found", BillingPlanNotFound, http.StatusNotFound, "BILLING.PLAN_NOT_FOUND"},
		{"billing rate limited", BillingCheckoutRateLimited, http.StatusTooManyRequests, "BILLING.CHECKOUT_RATE_LIMITED"},
		{"billing invoice paid", BillingInvoiceAlreadyPaid, http.StatusConflict, "BILLING.INVOICE_ALREADY_PAID"},
		{"multisub unavailable", MultiSubRemnawaveUnavailable, http.StatusServiceUnavailable, "MULTISUB.REMNAWAVE_UNAVAILABLE"},
		{"payment failed", PaymentFailed, http.StatusBadGateway, "PAYMENT.FAILED"},
		{"payment no plugin", PaymentNoPlugin, http.StatusServiceUnavailable, "PAYMENT.NO_PLUGIN"},
		{"reseller dup domain", ResellerDuplicateDomain, http.StatusConflict, "RESELLER.DUPLICATE_DOMAIN"},
		{"reseller invalid key", ResellerInvalidAPIKey, http.StatusUnauthorized, "RESELLER.INVALID_API_KEY"},
		{"plugin not found", PluginNotFound, http.StatusNotFound, "PLUGIN.NOT_FOUND"},
		{"plugin wasm fail", PluginWASMCompileFail, http.StatusUnprocessableEntity, "PLUGIN.WASM_COMPILATION_FAILED"},
		{"plugin draining", PluginDraining, http.StatusServiceUnavailable, "PLUGIN.DRAINING"},
		{"common validation", ValidationFailed, http.StatusUnprocessableEntity, "COMMON.VALIDATION_ERROR"},
		{"common not found", NotFound, http.StatusNotFound, "COMMON.NOT_FOUND"},
		{"common internal", Internal, http.StatusInternalServerError, "COMMON.INTERNAL"},
		{"common body too large", BodyTooLarge, http.StatusRequestEntityTooLarge, "COMMON.BODY_TOO_LARGE"},
		{"common unauthorized", Unauthorized, http.StatusUnauthorized, "COMMON.UNAUTHORIZED"},
		{"common forbidden", Forbidden, http.StatusForbidden, "COMMON.FORBIDDEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantStatus, tt.err.HTTPStatus)
			assert.Equal(t, tt.wantCode, tt.err.Code)
			assert.NotEmpty(t, tt.err.Message)
		})
	}
}
