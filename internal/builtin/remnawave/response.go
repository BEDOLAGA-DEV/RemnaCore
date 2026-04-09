package remnawave

import (
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
