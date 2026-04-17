package checkout

import (
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeAPIError(w http.ResponseWriter, apiErr *apierror.Error) {
	writeJSON(w, apiErr.HTTPStatus, map[string]any{
		"error":   apiErr.Code,
		"message": apiErr.Message,
	})
}
