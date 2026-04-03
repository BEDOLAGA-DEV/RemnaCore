package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock PluginRepository ---

type mockRepo struct {
	plugins map[string]*Plugin // keyed by ID
	slugIdx map[string]string  // slug -> ID
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		plugins: make(map[string]*Plugin),
		slugIdx: make(map[string]string),
	}
}

func (r *mockRepo) Create(_ context.Context, p *Plugin) error {
	if _, exists := r.slugIdx[p.Slug]; exists {
		return ErrPluginAlreadyExists
	}
	r.plugins[p.ID] = p
	r.slugIdx[p.Slug] = p.ID
	return nil
}

func (r *mockRepo) GetByID(_ context.Context, id string) (*Plugin, error) {
	p, ok := r.plugins[id]
	if !ok {
		return nil, ErrPluginNotFound
	}
	// Return a copy to avoid mutation leaking.
	clone := *p
	return &clone, nil
}

func (r *mockRepo) GetBySlug(_ context.Context, slug string) (*Plugin, error) {
	id, ok := r.slugIdx[slug]
	if !ok {
		return nil, ErrPluginNotFound
	}
	return r.GetByID(context.Background(), id)
}

func (r *mockRepo) GetAll(_ context.Context) ([]*Plugin, error) {
	out := make([]*Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	return out, nil
}

func (r *mockRepo) GetEnabled(_ context.Context) ([]*Plugin, error) {
	var out []*Plugin
	for _, p := range r.plugins {
		if p.Status == StatusEnabled {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *mockRepo) UpdateStatus(_ context.Context, id string, status PluginStatus, errorLog string, enabledAt *time.Time) error {
	p, ok := r.plugins[id]
	if !ok {
		return ErrPluginNotFound
	}
	p.Status = status
	p.ErrorLog = errorLog
	p.EnabledAt = enabledAt
	p.UpdatedAt = time.Now()
	return nil
}

func (r *mockRepo) UpdateConfig(_ context.Context, id string, config map[string]string) error {
	p, ok := r.plugins[id]
	if !ok {
		return ErrPluginNotFound
	}
	p.Config = config
	p.UpdatedAt = time.Now()
	return nil
}

func (r *mockRepo) UpdatePlugin(_ context.Context, p *Plugin) error {
	existing, ok := r.plugins[p.ID]
	if !ok {
		return ErrPluginNotFound
	}
	// Preserve the slug index.
	if existing.Slug != p.Slug {
		delete(r.slugIdx, existing.Slug)
		r.slugIdx[p.Slug] = p.ID
	}
	r.plugins[p.ID] = p
	return nil
}

func (r *mockRepo) Delete(_ context.Context, id string) error {
	p, ok := r.plugins[id]
	if !ok {
		return ErrPluginNotFound
	}
	delete(r.slugIdx, p.Slug)
	delete(r.plugins, id)
	return nil
}

// --- Mock StorageService ---

type mockStorage struct {
	deleted map[string]bool // slug -> deleteAll called
}

func newMockStorage() *mockStorage {
	return &mockStorage{deleted: make(map[string]bool)}
}

func (s *mockStorage) Get(_ context.Context, _, _ string) ([]byte, error)    { return nil, nil }
func (s *mockStorage) Set(_ context.Context, _, _ string, _ []byte, _ int64) error { return nil }
func (s *mockStorage) Delete(_ context.Context, _, _ string) error           { return nil }
func (s *mockStorage) DeleteAll(_ context.Context, slug string) error {
	s.deleted[slug] = true
	return nil
}
func (s *mockStorage) GetUsedBytes(_ context.Context, _ string) (int64, error) { return 0, nil }

// --- Helpers ---

var validManifestTOML = []byte(`
[plugin]
id = "test-plugin"
name = "Test Plugin"
version = "1.0.0"
description = "A test plugin"
author = "tester"

[hooks]
sync = ["invoice.created"]
`)

var validManifestTOML2 = []byte(`
[plugin]
id = "another-plugin"
name = "Another Plugin"
version = "1.0.0"

[hooks]
async = ["payment.completed"]
`)

func newTestLifecycleManager() (*LifecycleManager, *mockRepo, *mockStorage, *testPublisher) {
	repo := newMockRepo()
	storage := newMockStorage()
	pub := &testPublisher{}
	logger := testErrorLogger()

	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return []byte(`{"action":"continue"}`), nil
		}}, nil
	}

	runtime := NewRuntimePool(logger, factory)
	dispatcher := NewHookDispatcher(runtime, pub, logger)

	lm := NewLifecycleManager(repo, storage, runtime, dispatcher, pub, logger)
	return lm, repo, storage, pub
}

// --- Tests ---

func TestInstall_Success(t *testing.T) {
	lm, repo, _, pub := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)
	require.NotNil(t, p)

	assert.Equal(t, "test-plugin", p.Slug)
	assert.Equal(t, StatusInstalled, p.Status)
	assert.NotEmpty(t, p.ID)

	// Plugin should be in repo.
	stored, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", stored.Slug)

	// Installed event should be published.
	require.Len(t, pub.events, 1)
	assert.Equal(t, EventPluginInstalled, pub.events[0].Type)
}

func TestInstall_DuplicateSlug(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	_, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	_, err = lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginAlreadyExists)
}

func TestInstall_InvalidManifest(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	_, err := lm.Install(ctx, []byte("not valid toml {{{"), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestEnable_Success(t *testing.T) {
	lm, _, _, pub := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	pub.events = nil // Reset events.

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	// Plugin should now be loaded in runtime.
	slugs := lm.runtime.LoadedSlugs()
	assert.Contains(t, slugs, "test-plugin")

	// Hooks should be registered.
	regs := lm.dispatcher.Registrations("invoice.created")
	assert.Len(t, regs, 1)

	// Enabled event should be published.
	require.Len(t, pub.events, 1)
	assert.Equal(t, EventPluginEnabled, pub.events[0].Type)
}

func TestEnable_AlreadyEnabled(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginAlreadyEnabled)
}

func TestDisable_Success(t *testing.T) {
	lm, _, _, pub := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	pub.events = nil

	err = lm.Disable(ctx, p.ID)
	require.NoError(t, err)

	// Plugin should be unloaded.
	slugs := lm.runtime.LoadedSlugs()
	assert.NotContains(t, slugs, "test-plugin")

	// Hooks should be unregistered.
	regs := lm.dispatcher.Registrations("invoice.created")
	assert.Empty(t, regs)

	// Disabled event published.
	require.Len(t, pub.events, 1)
	assert.Equal(t, EventPluginDisabled, pub.events[0].Type)
}

func TestUninstall_Success(t *testing.T) {
	lm, repo, storage, pub := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	pub.events = nil

	err = lm.Uninstall(ctx, p.ID)
	require.NoError(t, err)

	// Storage should have been deleted.
	assert.True(t, storage.deleted["test-plugin"])

	// Plugin should be removed from repo.
	_, err = repo.GetByID(ctx, p.ID)
	require.ErrorIs(t, err, ErrPluginNotFound)

	// Uninstalled event published (after disabled event).
	var foundUninstalled bool
	for _, e := range pub.events {
		if e.Type == EventPluginUninstalled {
			foundUninstalled = true
		}
	}
	assert.True(t, foundUninstalled)
}

func TestLoadAllEnabled_Success(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	// Install and enable two plugins.
	p1, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm-1"))
	require.NoError(t, err)
	err = lm.Enable(ctx, p1.ID)
	require.NoError(t, err)

	p2, err := lm.Install(ctx, validManifestTOML2, []byte("fake-wasm-2"))
	require.NoError(t, err)
	err = lm.Enable(ctx, p2.ID)
	require.NoError(t, err)

	// Create a fresh runtime pool (simulating restart).
	logger := testErrorLogger()
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{}, nil
	}
	freshRuntime := NewRuntimePool(logger, factory)
	freshDispatcher := NewHookDispatcher(freshRuntime, &testPublisher{}, logger)

	lm.runtime = freshRuntime
	lm.dispatcher = freshDispatcher

	err = lm.LoadAllEnabled(ctx)
	require.NoError(t, err)

	slugs := freshRuntime.LoadedSlugs()
	assert.Contains(t, slugs, "test-plugin")
	assert.Contains(t, slugs, "another-plugin")
}

func TestUpdateConfig_Success(t *testing.T) {
	lm, repo, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	manifestWithConfig := []byte(`
[plugin]
id = "configurable-plugin"
name = "Configurable"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]

[config.api_key]
type = "secret"
label = "API Key"
required = true
`)

	p, err := lm.Install(ctx, manifestWithConfig, []byte("wasm"))
	require.NoError(t, err)

	err = lm.UpdateConfig(ctx, p.ID, map[string]string{"api_key": "sk-test-123"})
	require.NoError(t, err)

	stored, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "sk-test-123", stored.Config["api_key"])
}

func TestUpdateConfig_MissingRequired(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	manifestWithConfig := []byte(`
[plugin]
id = "required-config-plugin"
name = "Required Config"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]

[config.api_key]
type = "secret"
label = "API Key"
required = true
`)

	p, err := lm.Install(ctx, manifestWithConfig, []byte("wasm"))
	require.NoError(t, err)

	// Pass empty config — should fail because api_key is required.
	err = lm.UpdateConfig(ctx, p.ID, map[string]string{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestInstall_ManifestMissingHooks(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	badManifest := []byte(`
[plugin]
id = "no-hooks"
name = "No Hooks"
version = "1.0.0"
`)

	_, err := lm.Install(ctx, badManifest, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestEnable_NotFound(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	err := lm.Enable(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPluginNotFound))
}

// --- Hot Reload Tests ---

var hotReloadManifestV2 = []byte(`
[plugin]
id = "test-plugin"
name = "Test Plugin"
version = "2.0.0"
description = "Updated test plugin"
author = "tester"

[hooks]
sync = ["invoice.created", "payment.completed"]
`)

func TestHotReload_Success(t *testing.T) {
	lm, repo, _, pub := newTestLifecycleManager()
	ctx := context.Background()

	// Install and enable v1.
	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm-v1"))
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	pub.events = nil

	// Hot reload to v2.
	err = lm.HotReload(ctx, p.ID, hotReloadManifestV2, []byte("fake-wasm-v2"))
	require.NoError(t, err)

	// Verify version updated in repo.
	stored, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", stored.Version)
	assert.Equal(t, "Updated test plugin", stored.Description)
	assert.Equal(t, StatusEnabled, stored.Status)

	// Verify old hooks replaced with new hooks.
	invoiceRegs := lm.dispatcher.Registrations("invoice.created")
	assert.Len(t, invoiceRegs, 1)

	paymentRegs := lm.dispatcher.Registrations("payment.completed")
	assert.Len(t, paymentRegs, 1)

	// Verify plugin is still loaded in runtime.
	slugs := lm.runtime.LoadedSlugs()
	assert.Contains(t, slugs, "test-plugin")

	// Verify hot_reloaded event published.
	require.Len(t, pub.events, 1)
	assert.Equal(t, EventPluginHotReloaded, pub.events[0].Type)
	assert.Equal(t, "1.0.0", pub.events[0].Data["old_version"])
	assert.Equal(t, "2.0.0", pub.events[0].Data["new_version"])
}

func TestHotReload_SlugMismatch(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	// Try to hot reload with a manifest that has a different slug.
	differentSlugManifest := []byte(`
[plugin]
id = "different-plugin"
name = "Different Plugin"
version = "2.0.0"

[hooks]
sync = ["invoice.created"]
`)

	err = lm.HotReload(ctx, p.ID, differentSlugManifest, []byte("fake-wasm-v2"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSlugMismatch)
}

func TestHotReload_PluginNotEnabled(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	// Install but do NOT enable.
	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	err = lm.HotReload(ctx, p.ID, hotReloadManifestV2, []byte("fake-wasm-v2"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginNotRunning)
}

func TestHotReload_PluginNotFound(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	err := lm.HotReload(ctx, "nonexistent-id", hotReloadManifestV2, []byte("fake-wasm-v2"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestHotReload_InvalidManifest(t *testing.T) {
	lm, _, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	p, err := lm.Install(ctx, validManifestTOML, []byte("fake-wasm"))
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	err = lm.HotReload(ctx, p.ID, []byte("not valid toml {{{"), []byte("fake-wasm-v2"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestHotReload_PreservesConfig(t *testing.T) {
	lm, repo, _, _ := newTestLifecycleManager()
	ctx := context.Background()

	manifestWithConfig := []byte(`
[plugin]
id = "config-plugin"
name = "Config Plugin"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]

[config.api_key]
type = "secret"
label = "API Key"
required = true
`)

	p, err := lm.Install(ctx, manifestWithConfig, []byte("wasm"))
	require.NoError(t, err)

	err = lm.UpdateConfig(ctx, p.ID, map[string]string{"api_key": "sk-test-123"})
	require.NoError(t, err)

	err = lm.Enable(ctx, p.ID)
	require.NoError(t, err)

	v2Manifest := []byte(`
[plugin]
id = "config-plugin"
name = "Config Plugin"
version = "2.0.0"

[hooks]
sync = ["invoice.created", "payment.completed"]

[config.api_key]
type = "secret"
label = "API Key"
required = true
`)

	err = lm.HotReload(ctx, p.ID, v2Manifest, []byte("wasm-v2"))
	require.NoError(t, err)

	stored, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", stored.Version)
	assert.Equal(t, "sk-test-123", stored.Config["api_key"])
}
