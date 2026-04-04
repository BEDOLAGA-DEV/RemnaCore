package integration_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	shutdownTestHandlerDelay = 500 * time.Millisecond
	shutdownTestTimeout      = 2 * time.Second
	shutdownTestSettleDelay  = 100 * time.Millisecond
)

// TestGracefulShutdown_InFlightRequestCompletes verifies that the HTTP server's
// graceful shutdown allows in-flight requests to complete before closing.
//
// We cannot easily test the full Fx lifecycle here, but we can verify that
// net/http.Server.Shutdown behaves correctly with our handler pattern.
func TestGracefulShutdown_InFlightRequestCompletes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shutdown test in short mode")
	}

	// Create a slow handler that simulates an in-flight request.
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(shutdownTestHandlerDelay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"completed"}`))
	})

	server := &http.Server{
		Handler: slowHandler,
	}

	// Start server on a random port.
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	go func() { _ = server.Serve(listener) }()
	defer server.Close() //nolint:errcheck

	addr := listener.Addr().String()

	// Start an in-flight request.
	resultCh := make(chan int, 1)
	go func() {
		resp, reqErr := http.Get("http://" + addr + "/")
		if reqErr != nil {
			resultCh <- 0
			return
		}
		defer resp.Body.Close()
		resultCh <- resp.StatusCode
	}()

	// Give the request time to reach the handler.
	time.Sleep(shutdownTestSettleDelay)

	// Initiate graceful shutdown.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTestTimeout)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	assert.NoError(t, err)

	// The in-flight request should have completed successfully.
	status := <-resultCh
	assert.Equal(t, http.StatusOK, status)
}
