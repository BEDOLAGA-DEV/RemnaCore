package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker/v2"
	"golang.org/x/sync/singleflight"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// Subscription proxy constants.
const (
	SubscriptionProxyPort = 4100
	L1CacheSize           = 10000 // maximum in-memory cache entries
	L1CacheTTL            = 5 * time.Minute
	L2CacheTTL            = 15 * time.Minute

	subscriptionProxyReadTimeout  = 10 * time.Second
	subscriptionProxyWriteTimeout = 10 * time.Second

	// ProxyHTTPTimeout is the timeout for outbound HTTP requests to Remnawave
	// when fetching subscription configs.
	ProxyHTTPTimeout = 10 * time.Second

	// ProxyCBName is the circuit breaker instance name for the subscription proxy.
	ProxyCBName = "subscription-proxy-remnawave"

	// ProxyCBMaxRequests is the number of probe requests allowed in the
	// half-open state.
	ProxyCBMaxRequests = 1

	// ProxyCBTimeout is the duration the circuit stays open before
	// transitioning to half-open.
	ProxyCBTimeout = 15 * time.Second

	// ProxyCBConsecutiveFailures is the number of consecutive failures
	// required to trip the breaker from closed to open.
	ProxyCBConsecutiveFailures = 5

	// L1CacheName is the Prometheus label value for the L1 in-memory LRU cache.
	L1CacheName = "subscription_proxy_l1"

	// L2CacheKeyPrefix is the Valkey key prefix for L2-cached subscription configs.
	L2CacheKeyPrefix = "sub:"
	// RemnawaveSubPath is the URL path segment for Remnawave subscription endpoints.
	RemnawaveSubPath = "/sub/"

	// MaxSubscriptionConfigBytes is the maximum allowed size for a subscription
	// configuration response from Remnawave.
	MaxSubscriptionConfigBytes = 1 << 20 // 1 MB

	// ShutdownTimeout is the maximum time allowed for graceful HTTP server shutdown.
	ShutdownTimeout = 5 * time.Second
)

// SubscriptionProxy serves VPN subscription configs to clients. It implements a
// three-tier cache: L1 (in-memory LRU) -> L2 (Valkey) -> L3 (Remnawave API).
//
// L3 fetches are deduplicated via singleflight and protected by a circuit
// breaker that opens after ProxyCBConsecutiveFailures consecutive failures.
type SubscriptionProxy struct {
	remnawaveClient *remnawave.Client
	httpClient      *http.Client
	l1Cache         *LRUCache
	valkeyClient    *redis.Client
	logger          *slog.Logger
	clock           clock.Clock
	metrics         *observability.Metrics
	sfGroup         singleflight.Group
	cbRemnawave     *gobreaker.CircuitBreaker[[]byte]
}

// NewSubscriptionProxy creates a SubscriptionProxy wired to the given backends.
func NewSubscriptionProxy(
	client *remnawave.Client,
	valkeyClient *redis.Client,
	logger *slog.Logger,
	clk clock.Clock,
	metrics *observability.Metrics,
) *SubscriptionProxy {
	cb := gobreaker.NewCircuitBreaker[[]byte](gobreaker.Settings{
		Name:        ProxyCBName,
		MaxRequests: ProxyCBMaxRequests,
		Timeout:     ProxyCBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= ProxyCBConsecutiveFailures
		},
	})

	return &SubscriptionProxy{
		remnawaveClient: client,
		httpClient: &http.Client{
			Timeout: ProxyHTTPTimeout,
		},
		l1Cache:      NewLRUCache(L1CacheSize, L1CacheName, metrics),
		valkeyClient: valkeyClient,
		logger:       logger,
		clock:        clk,
		metrics:      metrics,
		cbRemnawave:  cb,
	}
}

// ServeSubscription handles GET /{shortUuid}. It looks up the subscription
// config through the L1 -> L2 -> L3 cache chain and returns it to the VPN client.
func (sp *SubscriptionProxy) ServeSubscription(w http.ResponseWriter, r *http.Request) {
	start := sp.clock.Now()
	sp.incActiveConnections()
	defer sp.decActiveConnections()

	shortUUID := chi.URLParam(r, "shortUuid")
	if shortUUID == "" {
		sp.observeRequest(start, http.StatusBadRequest)
		http.Error(w, "missing shortUuid", http.StatusBadRequest)
		return
	}

	// L1: in-memory LRU cache.
	if body, ok := sp.l1Cache.Get(shortUUID, sp.clock.Now()); ok {
		sp.writeSubscriptionResponse(w, body)
		sp.observeRequest(start, http.StatusOK)
		return
	}

	// L2: Valkey cache.
	l2Key := L2CacheKeyPrefix + shortUUID
	l2Data, err := sp.valkeyClient.Get(r.Context(), l2Key).Bytes()
	if err == nil {
		// Populate L1 from L2.
		sp.l1Cache.Set(shortUUID, l2Data, sp.clock.Now().Add(L1CacheTTL))
		sp.writeSubscriptionResponse(w, l2Data)
		sp.observeRequest(start, http.StatusOK)
		return
	}

	// L3: Singleflight + Circuit Breaker.
	result, err, shared := sp.sfGroup.Do(shortUUID, func() (any, error) {
		return sp.cbRemnawave.Execute(func() ([]byte, error) {
			return sp.fetchFromRemnawave(r.Context(), shortUUID)
		})
	})
	if err != nil {
		sp.logger.Error("subscription proxy: remnawave fetch failed",
			slog.String("short_uuid", shortUUID),
			slog.Any("error", err),
		)
		sp.incUpstreamErrors()
		sp.observeRequest(start, http.StatusBadGateway)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	body := result.([]byte)
	if shared {
		sp.logger.Debug("singleflight: shared L3 result", slog.String("uuid", shortUUID))
	}

	// Store in L2 and L1.
	sp.valkeyClient.Set(r.Context(), l2Key, body, L2CacheTTL)
	sp.l1Cache.Set(shortUUID, body, sp.clock.Now().Add(L1CacheTTL))

	sp.writeSubscriptionResponse(w, body)
	sp.observeRequest(start, http.StatusOK)
}

// fetchFromRemnawave retrieves the subscription config from Remnawave by
// calling the subscription URL endpoint directly.
func (sp *SubscriptionProxy) fetchFromRemnawave(ctx context.Context, shortUUID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		sp.remnawaveClient.BaseURL()+RemnawaveSubPath+shortUUID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := sp.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxSubscriptionConfigBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > MaxSubscriptionConfigBytes {
		return nil, fmt.Errorf("subscription config exceeds %d bytes", MaxSubscriptionConfigBytes)
	}

	return body, nil
}

// writeSubscriptionResponse writes the cached subscription config to the
// response.
func (sp *SubscriptionProxy) writeSubscriptionResponse(w http.ResponseWriter, body []byte) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypePlain)
	w.Header().Set(httpconst.HeaderCacheControl, httpconst.CacheControlNoStore)
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

// incActiveConnections increments the active connections gauge.
func (sp *SubscriptionProxy) incActiveConnections() {
	if sp.metrics != nil {
		sp.metrics.ProxyActiveConnections.Inc()
	}
}

// decActiveConnections decrements the active connections gauge.
func (sp *SubscriptionProxy) decActiveConnections() {
	if sp.metrics != nil {
		sp.metrics.ProxyActiveConnections.Dec()
	}
}

// observeRequest records the request duration with the response status code.
func (sp *SubscriptionProxy) observeRequest(start time.Time, statusCode int) {
	if sp.metrics != nil {
		duration := sp.clock.Now().Sub(start).Seconds()
		sp.metrics.ProxyRequestDuration.WithLabelValues(strconv.Itoa(statusCode)).Observe(duration)
	}
}

// incUpstreamErrors increments the upstream error counter.
func (sp *SubscriptionProxy) incUpstreamErrors() {
	if sp.metrics != nil {
		sp.metrics.ProxyUpstreamErrors.Inc()
	}
}

// Start begins listening on the given port in a separate HTTP server. It blocks
// until ctx is cancelled.
func (sp *SubscriptionProxy) Start(ctx context.Context, port int) error {
	r := chi.NewRouter()
	r.Get("/{shortUuid}", sp.ServeSubscription)

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  subscriptionProxyReadTimeout,
		WriteTimeout: subscriptionProxyWriteTimeout,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("subscription proxy: bind %s: %w", addr, err)
	}

	sp.logger.Info("subscription proxy starting", slog.String("addr", addr))

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	<-ctx.Done()
	sp.logger.Info("subscription proxy shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
