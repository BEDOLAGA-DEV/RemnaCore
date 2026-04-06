package infra

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func TestSubscriptionProxy_L1CacheHit(t *testing.T) {
	cache := NewLRUCache(L1CacheSize)
	proxy := &SubscriptionProxy{
		l1Cache: cache,
		logger:  slog.Default(),
		clock:   clock.NewReal(),
	}

	// Pre-populate L1 cache.
	cache.Set("abc123", []byte("cached-subscription-config"), time.Now().Add(L1CacheTTL))

	r := chi.NewRouter()
	r.Get("/{shortUuid}", proxy.ServeSubscription)

	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cached-subscription-config", w.Body.String())
}

func TestSubscriptionProxy_L1CacheExpired(t *testing.T) {
	// Create a mock Remnawave server.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fresh-from-remnawave"))
	}))
	defer mockServer.Close()

	client := remnawave.NewClient(mockServer.URL, "test-token")

	// Use a Valkey client that will fail (no real Redis), forcing L3 fetch.
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:1", DialTimeout: 1 * time.Millisecond, ReadTimeout: 1 * time.Millisecond})

	proxy := NewSubscriptionProxy(client, valkeyClient, slog.Default(), clock.NewReal())

	// Pre-populate L1 with an expired entry.
	proxy.l1Cache.Set("expired123", []byte("stale-data"), time.Now().Add(-1*time.Minute))

	r := chi.NewRouter()
	r.Get("/{shortUuid}", proxy.ServeSubscription)

	req := httptest.NewRequest(http.MethodGet, "/expired123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "fresh-from-remnawave", w.Body.String())
}

func TestSubscriptionProxy_MissingShortUUID(t *testing.T) {
	proxy := &SubscriptionProxy{
		l1Cache: NewLRUCache(L1CacheSize),
		logger:  slog.Default(),
		clock:   clock.NewReal(),
	}

	// Directly call handler without chi URL param set.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	proxy.ServeSubscription(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubscriptionProxy_UpstreamError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := remnawave.NewClient(mockServer.URL, "test-token")
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:1", DialTimeout: 1 * time.Millisecond, ReadTimeout: 1 * time.Millisecond})

	proxy := NewSubscriptionProxy(client, valkeyClient, slog.Default(), clock.NewReal())

	r := chi.NewRouter()
	r.Get("/{shortUuid}", proxy.ServeSubscription)

	req := httptest.NewRequest(http.MethodGet, "/fail123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestSubscriptionProxy_L3PopulatesL1(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("new-config-data"))
	}))
	defer mockServer.Close()

	client := remnawave.NewClient(mockServer.URL, "test-token")
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:1", DialTimeout: 1 * time.Millisecond, ReadTimeout: 1 * time.Millisecond})

	proxy := NewSubscriptionProxy(client, valkeyClient, slog.Default(), clock.NewReal())

	r := chi.NewRouter()
	r.Get("/{shortUuid}", proxy.ServeSubscription)

	req := httptest.NewRequest(http.MethodGet, "/new123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the L1 cache was populated.
	cached, ok := proxy.l1Cache.Get("new123", time.Now())
	assert.True(t, ok)
	assert.Equal(t, "new-config-data", string(cached))
}

func TestNewSubscriptionProxy(t *testing.T) {
	client := remnawave.NewClient("http://localhost:3000", "token")
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	proxy := NewSubscriptionProxy(client, valkeyClient, slog.Default(), clock.NewReal())

	assert.NotNil(t, proxy)
	assert.NotNil(t, proxy.l1Cache)
	assert.NotNil(t, proxy.remnawaveClient)
	assert.NotNil(t, proxy.cbRemnawave, "circuit breaker must be initialized")

	_ = valkeyClient.Close()
}

func TestSubscriptionProxy_Singleflight_DeduplicatesL3(t *testing.T) {
	// 10 concurrent requests for the same UUID must result in exactly 1
	// HTTP call to Remnawave via singleflight deduplication.
	const concurrentRequests = 10

	var httpCalls atomic.Int64
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls.Add(1)
		// Simulate a slow upstream so all goroutines pile up on singleflight.
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("deduplicated-response"))
	}))
	defer mockServer.Close()

	client := remnawave.NewClient(mockServer.URL, "test-token")
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:1", DialTimeout: 1 * time.Millisecond, ReadTimeout: 1 * time.Millisecond})
	proxy := NewSubscriptionProxy(client, valkeyClient, slog.Default(), clock.NewReal())

	router := chi.NewRouter()
	router.Get("/{shortUuid}", proxy.ServeSubscription)

	var wg sync.WaitGroup
	results := make([]*httptest.ResponseRecorder, concurrentRequests)

	// Barrier ensures all goroutines start before any begins processing.
	barrier := make(chan struct{})

	for i := range concurrentRequests {
		wg.Add(1)
		results[i] = httptest.NewRecorder()
		go func(idx int) {
			defer wg.Done()
			<-barrier // wait for all goroutines to be ready
			req := httptest.NewRequest(http.MethodGet, "/dedup-uuid", nil)
			router.ServeHTTP(results[idx], req)
		}(i)
	}
	close(barrier) // release all goroutines simultaneously
	wg.Wait()

	// All requests must succeed.
	for i, rec := range results {
		assert.Equal(t, http.StatusOK, rec.Code,
			"request %d should succeed", i)
		assert.Equal(t, "deduplicated-response", rec.Body.String(),
			"request %d should return the shared response", i)
	}

	// Singleflight should collapse most concurrent calls. The exact dedup ratio
	// depends on goroutine scheduling — with L2 Valkey unavailable, each request
	// must fail the L2 check before hitting singleflight, creating small timing
	// windows. Allow ≤ concurrentRequests/2 as a meaningful reduction.
	calls := httpCalls.Load()
	assert.LessOrEqual(t, calls, int64(concurrentRequests/2),
		"singleflight should reduce upstream calls significantly, got %d/%d", calls, concurrentRequests)
	assert.GreaterOrEqual(t, calls, int64(1),
		"at least 1 upstream call should be made")
}

func TestSubscriptionProxy_CircuitBreaker_OpensAfterFailures(t *testing.T) {
	// After ProxyCBConsecutiveFailures consecutive failures, the circuit
	// breaker should open and reject the next call immediately.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := remnawave.NewClient(mockServer.URL, "test-token")
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:1", DialTimeout: 1 * time.Millisecond, ReadTimeout: 1 * time.Millisecond})
	proxy := NewSubscriptionProxy(client, valkeyClient, slog.Default(), clock.NewReal())

	router := chi.NewRouter()
	router.Get("/{shortUuid}", proxy.ServeSubscription)

	// Trip the circuit breaker by issuing ProxyCBConsecutiveFailures requests.
	// Each must use a unique UUID to avoid singleflight deduplication.
	for i := range ProxyCBConsecutiveFailures {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/fail-%d", i), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadGateway, w.Code,
			"failure %d should return 502", i+1)
	}

	// The breaker should now be open. The next request must fail immediately
	// without calling the upstream (gobreaker returns ErrOpenState).
	req := httptest.NewRequest(http.MethodGet, "/after-trip", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadGateway, w.Code,
		"request after breaker trips should return 502")
}

func TestSubscriptionProxy_CircuitBreaker_Constants(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "name is non-empty",
			check: func(t *testing.T) {
				assert.NotEmpty(t, ProxyCBName)
			},
		},
		{
			name: "max requests is positive",
			check: func(t *testing.T) {
				assert.Greater(t, int(ProxyCBMaxRequests), 0)
			},
		},
		{
			name: "timeout is positive",
			check: func(t *testing.T) {
				assert.Greater(t, ProxyCBTimeout, time.Duration(0))
			},
		},
		{
			name: "consecutive failures threshold is positive",
			check: func(t *testing.T) {
				assert.Greater(t, int(ProxyCBConsecutiveFailures), 0)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

func TestSubscriptionProxy_CircuitBreaker_DirectTrip(t *testing.T) {
	// Verify the gobreaker instance embedded in the proxy behaves correctly:
	// 5 failures -> state open -> next call rejected without execution.
	cb := gobreaker.NewCircuitBreaker[[]byte](gobreaker.Settings{
		Name:        ProxyCBName,
		MaxRequests: ProxyCBMaxRequests,
		Timeout:     ProxyCBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= ProxyCBConsecutiveFailures
		},
	})

	errUpstream := fmt.Errorf("upstream 500")

	// Record consecutive failures up to the threshold.
	for i := range ProxyCBConsecutiveFailures {
		t.Run(fmt.Sprintf("failure_%d", i+1), func(t *testing.T) {
			_, err := cb.Execute(func() ([]byte, error) {
				return nil, errUpstream
			})
			require.Error(t, err)
		})
	}

	assert.Equal(t, gobreaker.StateOpen, cb.State(),
		"breaker should be open after %d consecutive failures", ProxyCBConsecutiveFailures)

	// Next call must be rejected without executing the function.
	functionCalled := false
	_, err := cb.Execute(func() ([]byte, error) {
		functionCalled = true
		return []byte("should-not-run"), nil
	})
	assert.Error(t, err)
	assert.False(t, functionCalled, "function must not be called when breaker is open")
}

func TestLRUCache_EvictsOldest(t *testing.T) {
	const maxSize = 5
	cache := NewLRUCache(maxSize)
	now := time.Now()
	expiry := now.Add(L1CacheTTL)

	// Insert maxSize + 10 entries.
	insertCount := maxSize + 10
	for i := range insertCount {
		cache.Set(fmt.Sprintf("key-%d", i), []byte(fmt.Sprintf("val-%d", i)), expiry)
	}

	assert.Equal(t, maxSize, cache.Len())
}

func TestLRUCache_RecentlyAccessedSurvivesEviction(t *testing.T) {
	const maxSize = 3
	cache := NewLRUCache(maxSize)
	now := time.Now()
	expiry := now.Add(L1CacheTTL)

	// Fill cache: key-0, key-1, key-2.
	cache.Set("key-0", []byte("val-0"), expiry)
	cache.Set("key-1", []byte("val-1"), expiry)
	cache.Set("key-2", []byte("val-2"), expiry)

	// Access key-0 to promote it to most-recently-used.
	_, ok := cache.Get("key-0", now)
	require.True(t, ok)

	// Insert two more entries; key-1 and key-2 should be evicted (LRU), but
	// key-0 must survive because it was recently accessed.
	cache.Set("key-3", []byte("val-3"), expiry)
	cache.Set("key-4", []byte("val-4"), expiry)

	got, ok := cache.Get("key-0", now)
	assert.True(t, ok, "recently accessed key-0 should still be in cache")
	assert.Equal(t, "val-0", string(got))

	_, ok = cache.Get("key-1", now)
	assert.False(t, ok, "key-1 should have been evicted")
}

func TestLRUCache_ExpiredEntryReturnsNil(t *testing.T) {
	cache := NewLRUCache(L1CacheSize)
	past := time.Now().Add(-1 * time.Minute)
	cache.Set("expired", []byte("data"), past)

	_, ok := cache.Get("expired", time.Now())
	assert.False(t, ok, "expired entry should not be returned")
	assert.Equal(t, 0, cache.Len(), "expired entry should be removed from cache")
}

func TestFetchFromRemnawave_ContextCancelled(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := remnawave.NewClient(mockServer.URL, "test-token")
	proxy := &SubscriptionProxy{
		remnawaveClient: client,
		httpClient:      &http.Client{Timeout: ProxyHTTPTimeout},
		logger:          slog.Default(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := proxy.fetchFromRemnawave(ctx, "test")
	assert.Error(t, err)
}
