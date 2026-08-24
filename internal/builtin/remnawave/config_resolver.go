package remnawave

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// resolverCacheTTL bounds how long a resolved configuration is reused, and so
// also bounds how long an administrator waits for a change to take effect.
// Re-reading two rows on every webhook and every subscription request would be
// wasteful; half a minute of staleness is the compromise.
const resolverCacheTTL = 30 * time.Second

// PlatformConfig is the Remnawave configuration the platform itself needs,
// as opposed to the per-panel settings the admin UI works with.
//
// These values used to come from REMNAWAVE_* environment variables. That
// prefix is no longer loaded by the config package — Remnawave settings are
// administered through this plugin — so anything still reading them silently
// received an empty string. An empty base URL made the subscription proxy fail
// with `unsupported protocol scheme ""`, and an empty webhook secret made the
// handler fail closed and reject every inbound webhook with 403.
type PlatformConfig struct {
	// URL and APIToken address the panel the platform provisions against.
	// Taken from the first active panel connection, falling back to the
	// plugin's own config for installs that predate multi-panel support.
	URL      string
	APIToken string

	// WebhookSecret verifies the HMAC signature on inbound panel webhooks.
	// Declared in the manifest as the "Webhook Signing Secret" field.
	WebhookSecret string
}

// Configured reports whether the panel can actually be reached. Callers use it
// to tell "not set up yet" apart from "set up and failing".
func (c PlatformConfig) Configured() bool {
	return c.URL != "" && c.APIToken != ""
}

// ConfigResolver reads the remnawave-provider plugin's admin-managed
// configuration at call time. Resolving lazily rather than at construction is
// the point: the values live in the database and change while the process
// runs, so a value captured during dependency wiring would be stale — or, as
// was the case, permanently empty.
type ConfigResolver struct {
	collections pluginstore.Store
	plugins     plugin.PluginRepository
	clock       clock.Clock

	mu       sync.Mutex
	cached   PlatformConfig
	cachedAt time.Time
	hasCache bool
}

// NewConfigResolver creates a resolver over the plugin's stored configuration.
func NewConfigResolver(collections pluginstore.Store, plugins plugin.PluginRepository, clk clock.Clock) *ConfigResolver {
	return &ConfigResolver{collections: collections, plugins: plugins, clock: clk}
}

// Resolve returns the current platform-facing Remnawave configuration, cached
// for resolverCacheTTL.
func (r *ConfigResolver) Resolve(ctx context.Context) (PlatformConfig, error) {
	r.mu.Lock()
	if r.hasCache && r.clock.Now().Sub(r.cachedAt) < resolverCacheTTL {
		cfg := r.cached
		r.mu.Unlock()
		return cfg, nil
	}
	r.mu.Unlock()

	cfg, err := r.load(ctx)
	if err != nil {
		return PlatformConfig{}, err
	}

	r.mu.Lock()
	r.cached, r.cachedAt, r.hasCache = cfg, r.clock.Now(), true
	r.mu.Unlock()
	return cfg, nil
}

// Client builds a Remnawave client for the configured panel. It returns an
// error rather than a client with an empty base URL, so a misconfiguration
// surfaces as "panel not configured" instead of an opaque scheme error.
func (r *ConfigResolver) Client(ctx context.Context) (*rwclient.Client, error) {
	cfg, err := r.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Configured() {
		return nil, fmt.Errorf("remnawave panel is not configured: add a panel connection in the admin panel")
	}
	return rwclient.NewClient(cfg.URL, cfg.APIToken), nil
}

// load reads the configuration without consulting the cache.
//
// The read runs in platform scope. Panel connections are stored with a NULL
// tenant, and the row-level policy on plugins.collections only reveals those to
// the platform sentinel — so a caller without tenant context (the subscription
// proxy runs on its own listener, the webhook handler answers an unauthenticated
// request) would otherwise see an empty result and conclude nothing is
// configured.
func (r *ConfigResolver) load(ctx context.Context) (PlatformConfig, error) {
	ctx = tenantctx.WithPlatformScope(ctx)

	var cfg PlatformConfig

	// Plugin-level config first: the webhook secret lives there regardless of
	// how many panels are registered.
	p, err := r.plugins.GetBySlug(ctx, plugin.BuiltInSlugRemnawaveProvider)
	if err == nil && p != nil {
		cfg.WebhookSecret = p.Config[plugin.RemnawaveConfigKeyWebhookSecret]
		cfg.URL = p.Config[plugin.RemnawaveConfigKeyURL]
		cfg.APIToken = p.Config[plugin.RemnawaveConfigKeyAPIToken]
	}

	// An active panel connection wins over the legacy plugin-level fields.
	if panel, ok := r.activePanel(ctx); ok {
		cfg.URL, cfg.APIToken = panel.URL, panel.APIToken
	}

	return cfg, nil
}

// activePanel returns the first panel connection in active status.
func (r *ConfigResolver) activePanel(ctx context.Context) (PanelConnectionInput, bool) {
	docs, err := r.collections.ListDocuments(ctx, PluginSlug, CollectionPanelConnections)
	if err != nil {
		return PanelConnectionInput{}, false
	}
	for _, doc := range docs {
		var panel PanelConnectionInput
		if json.Unmarshal(doc.Data, &panel) != nil {
			continue
		}
		if panel.Status == PanelStatusActive && panel.URL != "" && panel.APIToken != "" {
			return panel, true
		}
	}
	return PanelConnectionInput{}, false
}

// WebhookSecret returns just the signing secret, for callers that verify
// inbound webhooks and have no use for the rest of the configuration. An
// unreadable configuration yields an empty secret, which the webhook handler
// treats as "reject everything" rather than "accept anything".
func (r *ConfigResolver) WebhookSecret(ctx context.Context) string {
	cfg, err := r.Resolve(ctx)
	if err != nil {
		return ""
	}
	return cfg.WebhookSecret
}
