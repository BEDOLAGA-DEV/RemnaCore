package infra

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
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
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:0"})

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
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:0"})

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
	valkeyClient := redis.NewClient(&redis.Options{Addr: "localhost:0"})

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

	_ = valkeyClient.Close()
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
