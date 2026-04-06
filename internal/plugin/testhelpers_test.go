package plugin

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// testPublisher is a mock event publisher shared across plugin tests.
type testPublisher struct {
	events []domainevent.Event
}

func (p *testPublisher) Publish(_ context.Context, event domainevent.Event) error {
	p.events = append(p.events, event)
	return nil
}

// syncTestPublisher is a thread-safe event publisher for tests that involve
// goroutines (e.g., DispatchAsync). Use this instead of testPublisher when
// events may be published from a background goroutine.
type syncTestPublisher struct {
	mu     sync.Mutex
	events []domainevent.Event
}

func (p *syncTestPublisher) Publish(_ context.Context, event domainevent.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

// Len returns the number of published events (thread-safe).
func (p *syncTestPublisher) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.events)
}

// Events returns a snapshot of published events (thread-safe).
func (p *syncTestPublisher) Events() []domainevent.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]domainevent.Event, len(p.events))
	copy(out, p.events)
	return out
}

// testErrorLogger returns a logger that only emits error-level messages,
// keeping test output clean.
func testErrorLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// testStorageService is an in-memory StorageService for unit tests.
type testStorageService struct {
	data map[string][]byte
}

func newTestStorageService() *testStorageService {
	return &testStorageService{data: make(map[string][]byte)}
}

func (s *testStorageService) Get(_ context.Context, pluginSlug, key string) ([]byte, error) {
	v, ok := s.data[pluginSlug+"/"+key]
	if !ok {
		return nil, ErrPluginNotFound
	}
	return v, nil
}

func (s *testStorageService) Set(_ context.Context, pluginSlug, key string, value []byte, _ int64) error {
	s.data[pluginSlug+"/"+key] = value
	return nil
}

func (s *testStorageService) Delete(_ context.Context, pluginSlug, key string) error {
	delete(s.data, pluginSlug+"/"+key)
	return nil
}

func (s *testStorageService) DeleteAll(_ context.Context, pluginSlug string) error {
	for k := range s.data {
		if len(k) > len(pluginSlug) && k[:len(pluginSlug)+1] == pluginSlug+"/" {
			delete(s.data, k)
		}
	}
	return nil
}

func (s *testStorageService) GetUsedBytes(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

// testPluginWithPerms creates a minimal Plugin with the given permission scopes
// and a non-nil manifest containing basic hooks (required for EffectiveLimits).
func testPluginWithPerms(perms []PermissionScope) *Plugin {
	return &Plugin{
		ID:   "test-id",
		Slug: "test-plugin",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{
				ID:         "test-plugin",
				Name:       "Test Plugin",
				Version:    "1.0.0",
				SDKVersion: CurrentSDKVersion,
			},
			Hooks: ManifestHooks{
				Sync: []string{"test_hook"},
			},
		},
		Permissions: perms,
	}
}
