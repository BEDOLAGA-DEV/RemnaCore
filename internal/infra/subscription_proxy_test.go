package infra

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func TestSubscriptionProxy_L1CacheHit(t *testing.T) {
	proxy := &SubscriptionProxy{
		l1Cache: &sync.Map{},
		logger:  slog.Default(),
		clock:   clock.NewReal(),
	}

	// Pre-populate L1 cache.
	proxy.l1Cache.Store("abc123", l1Entry{
		body:      []byte("cached-subscription-config"),
		expiresAt: time.Now().Add(5 * time.Minute),
	})

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
	proxy.l1Cache.Store("expired123", l1Entry{
		body:      []byte("stale-data"),
		expiresAt: time.Now().Add(-1 * time.Minute),
	})

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
		l1Cache: &sync.Map{},
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
	cached, ok := proxy.l1Cache.Load("new123")
	assert.True(t, ok)
	entry := cached.(l1Entry)
	assert.Equal(t, "new-config-data", string(entry.body))
	assert.True(t, entry.expiresAt.After(time.Now()))
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
