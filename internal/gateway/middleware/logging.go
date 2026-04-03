package middleware

import (
	"context"
	"log/slog"
	"net/http"
)

const (
	// loggerContextKeyValue is the typed context key for the request-scoped logger.
	loggerContextKeyValue contextKey = "request_logger"
)

// RequestLogger is middleware that creates a request-scoped slog.Logger with
// the request_id attached. It must be placed AFTER the RequestID middleware in
// the chain so that the request ID is available in the context.
//
// Handlers can retrieve the logger via LoggerFromContext(r.Context()).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, _ := r.Context().Value(RequestIDKey).(string)

		logger := slog.Default().With(slog.String("request_id", requestID))

		ctx := context.WithValue(r.Context(), loggerContextKeyValue, logger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggerFromContext returns the request-scoped logger from the context. If no
// logger is present (e.g. the request did not pass through RequestLogger
// middleware), slog.Default() is returned.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKeyValue).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
