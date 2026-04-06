package infra

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// Shared infrastructure constants.
const (
	// ShutdownTimeout is the maximum time allowed for graceful HTTP server
	// shutdown across all infrastructure services.
	ShutdownTimeout = 5 * time.Second
)

// Speed test server constants.
const (
	SpeedTestPort          = 4203
	SpeedTestRandomBufSize = 1 << 20   // 1 MB pre-allocated buffer
	MaxDownloadSize        = 100 << 20 // 100 MB max download
	DefaultChunkSize       = 64 << 10  // 64 KB write chunks
	SpeedTestReadTimeout   = 30 * time.Second
	SpeedTestWriteTimeout  = 60 * time.Second

	// SpeedTestRateLimit is the maximum number of speed test requests per IP
	// within SpeedTestRateLimitWindow.
	SpeedTestRateLimit = 10
	// SpeedTestRateLimitWindow is the sliding window duration for speed test
	// rate limiting.
	SpeedTestRateLimitWindow = time.Minute
)

// SpeedTestServer provides download, upload, and ping endpoints for client-side
// speed testing. It pre-allocates a random buffer at startup so that per-request
// crypto/rand overhead is avoided.
type SpeedTestServer struct {
	randomBuf []byte
	logger    *slog.Logger
}

// NewSpeedTestServer creates a SpeedTestServer with a pre-allocated random buffer.
func NewSpeedTestServer(logger *slog.Logger) (*SpeedTestServer, error) {
	buf := make([]byte, SpeedTestRandomBufSize)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("filling random buffer: %w", err)
	}
	return &SpeedTestServer{
		randomBuf: buf,
		logger:    logger,
	}, nil
}

// Download serves random bytes for download speed testing. The client may
// request a specific size via the "size" query parameter (bytes). The maximum
// is MaxDownloadSize.
func (s *SpeedTestServer) Download(w http.ResponseWriter, r *http.Request) {
	sizeStr := r.URL.Query().Get("size")
	size := SpeedTestRandomBufSize // default 1 MB
	if sizeStr != "" {
		parsed, err := strconv.Atoi(sizeStr)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid size parameter", http.StatusBadRequest)
			return
		}
		size = parsed
	}

	if size > MaxDownloadSize {
		size = MaxDownloadSize
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeOctetStream)
	w.Header().Set(httpconst.HeaderContentLength, strconv.Itoa(size))
	w.Header().Set(httpconst.HeaderCacheControl, httpconst.CacheControlNoStore)
	w.WriteHeader(http.StatusOK)

	written := 0
	bufLen := len(s.randomBuf)
	for written < size {
		chunk := DefaultChunkSize
		remaining := size - written
		if remaining < chunk {
			chunk = remaining
		}

		// Wrap around the pre-allocated buffer.
		offset := written % bufLen
		end := min(offset+chunk, bufLen)

		n, err := w.Write(s.randomBuf[offset:end])
		written += n
		if err != nil {
			return // client disconnected
		}
	}
}

// Upload consumes and discards incoming data for upload speed testing. The
// response reports the number of bytes received.
func (s *SpeedTestServer) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	buf := make([]byte, DefaultChunkSize)
	var total int64
	for {
		n, err := r.Body.Read(buf)
		total += int64(n)
		if err != nil {
			break
		}
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"bytes_received":%d}`, total)
}

// Ping returns a minimal response for latency measurement.
func (s *SpeedTestServer) Ping(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.Header().Set(httpconst.HeaderCacheControl, httpconst.CacheControlNoStore)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"pong":true}`)
}

// inProcessLimiter provides per-key rate limiting using a sliding window.
type inProcessLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	limit   int
	window  time.Duration
}

// newInProcessLimiter creates a rate limiter that allows at most limit requests
// per key within the given window duration.
func newInProcessLimiter(limit int, window time.Duration) *inProcessLimiter {
	return &inProcessLimiter{
		windows: make(map[string][]time.Time),
		limit:   limit,
		window:  window,
	}
}

// Allow reports whether the given key is within its rate limit. If allowed, the
// current timestamp is recorded.
func (l *inProcessLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Remove expired entries.
	timestamps := l.windows[key]
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}

	if len(valid) >= l.limit {
		l.windows[key] = valid
		return false
	}

	l.windows[key] = append(valid, now)
	return true
}

// withRateLimit wraps an http.HandlerFunc with per-IP rate limiting. Requests
// that exceed the limit receive a 429 Too Many Requests response.
func withRateLimit(limiter *inProcessLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !limiter.Allow(ip) {
			w.Header().Set(httpconst.HeaderRetryAfter, "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// Start begins listening on the given port in a separate HTTP server. It blocks
// until ctx is cancelled.
func (s *SpeedTestServer) Start(ctx context.Context, port int) error {
	limiter := newInProcessLimiter(SpeedTestRateLimit, SpeedTestRateLimitWindow)

	mux := http.NewServeMux()
	mux.HandleFunc("/download", withRateLimit(limiter, s.Download))
	mux.HandleFunc("/upload", withRateLimit(limiter, s.Upload))
	mux.HandleFunc("/ping", withRateLimit(limiter, s.Ping))

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  SpeedTestReadTimeout,
		WriteTimeout: SpeedTestWriteTimeout,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("speed test server: bind %s: %w", addr, err)
	}

	s.logger.Info("speed test server starting", slog.String("addr", addr))

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	<-ctx.Done()
	s.logger.Info("speed test server shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
