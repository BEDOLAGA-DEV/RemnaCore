package plugin

import (
	"context"
	"log/slog"
	"time"
)

// Health probe configuration.
const (
	// DefaultHealthProbeInterval is the default interval between health probes
	// across all loaded plugins.
	DefaultHealthProbeInterval = 5 * time.Minute

	// healthProbeFuncName is the WASM export name that plugins may implement
	// for proactive health checking. Plugins that do not export this function
	// will return a "function not found" error from CallHook, which the probe
	// treats as a no-op (best-effort).
	healthProbeFuncName = "__health_check"

	// healthProbeTimeout is the per-plugin timeout for a single health probe
	// call. Kept short to avoid blocking the probe loop.
	healthProbeTimeout = 5 * time.Second
)

// PluginHealthProbe periodically calls a lightweight probe on each loaded
// plugin to detect silent degradation (stuck state, memory leaks). If a
// probe fails, CallHook's corruption detection replaces broken runners
// automatically. The probe's primary value is surfacing warnings for
// monitoring and alerting.
type PluginHealthProbe struct {
	runtime  *RuntimePool
	logger   *slog.Logger
	interval time.Duration
}

// NewPluginHealthProbe creates a health probe that checks all loaded plugins
// at the default interval.
func NewPluginHealthProbe(runtime *RuntimePool, logger *slog.Logger) *PluginHealthProbe {
	return &PluginHealthProbe{
		runtime:  runtime,
		logger:   logger,
		interval: DefaultHealthProbeInterval,
	}
}

// Run starts the periodic health probe loop. It blocks until ctx is cancelled.
func (p *PluginHealthProbe) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.probeAll(ctx)
		}
	}
}

// probeAll calls the health check function on every loaded plugin. Failures
// are logged but do not disable the plugin. CallHook's corruption detection
// handles runner replacement automatically.
func (p *PluginHealthProbe) probeAll(ctx context.Context) {
	slugs := p.runtime.LoadedSlugs()
	for _, slug := range slugs {
		probeCtx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
		_, err := p.runtime.CallHook(probeCtx, slug, healthProbeFuncName, nil)
		cancel()

		if err != nil {
			// Health probe failed -- plugin may be degraded or may not export
			// the health check function. CallHook's corruption detection
			// replaces broken runners automatically. The probe's value is
			// surfacing the warning for monitoring.
			p.logger.Warn("plugin health probe failed",
				slog.String("slug", slug),
				slog.String("error", err.Error()),
			)
		}
	}
}
