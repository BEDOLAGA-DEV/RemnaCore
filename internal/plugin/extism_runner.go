package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	extism "github.com/extism/go-sdk"
)

// ExtismRunner implements WASMRunner using the Extism Go SDK backed by wazero.
type ExtismRunner struct {
	plugin *extism.Plugin
}

// applyManifestLimits configures resource limits and plugin config on an Extism
// manifest. This consolidates the shared logic between factory variants.
func applyManifestLimits(m *extism.Manifest, config map[string]string, limits ManifestLimits) {
	if limits.MaxFuel > 0 {
		m.Timeout = uint64(limits.MaxFuel)
	}
	if limits.MaxMemoryMB > 0 {
		m.Memory = &extism.ManifestMemory{
			MaxPages: uint32(limits.MaxMemoryMB * WASMPagesPerMB),
		}
	}
	if len(config) > 0 {
		m.Config = make(map[string]string, len(config))
		for k, v := range config {
			m.Config[k] = v
		}
	}
}

// ExtismRunnerFactory creates a WASMRunnerFactory that produces ExtismRunners
// with WASI support. The returned runners use wazero as the underlying runtime.
// Host functions bound to the given HostFunctions are registered so WASM guest
// code can call back into the host (e.g. log, kv_get, kv_set, http_request).
// Resource limits from the manifest are applied: MaxFuel is mapped to the
// Extism manifest timeout (the closest available control since wazero does not
// expose fuel-based CPU budgets); MaxMemoryMB is enforced via the manifest's
// MaxPages field (1 MB = 16 WASM pages of 64 KB each).
func ExtismRunnerFactory(hf *HostFunctions) WASMRunnerFactory {
	return func(slug string, wasmBytes []byte, config map[string]string, limits ManifestLimits) (WASMRunner, error) {
		manifest := extism.Manifest{
			Wasm: []extism.Wasm{
				extism.WasmData{Data: wasmBytes},
			},
		}

		applyManifestLimits(&manifest, config, limits)

		pluginConfig := extism.PluginConfig{
			EnableWasi: true,
		}

		// Build host functions bound to this plugin slug.
		hostFns := buildExtismHostFunctions(hf, slug)

		p, err := extism.NewPlugin(context.Background(), manifest, pluginConfig, hostFns)
		if err != nil {
			return nil, fmt.Errorf("create extism plugin: %w", err)
		}

		return &ExtismRunner{plugin: p}, nil
	}
}

// ExtismRunnerFactoryWithTimeout creates a WASMRunnerFactory that applies per-
// plugin timeout and resource limits from the effective manifest limits. The
// explicit timeoutMs overrides the limits.MaxFuel if non-zero. Memory limits
// from ManifestLimits.MaxMemoryMB are always enforced via the manifest's
// MaxPages field. Host functions are registered identically to
// ExtismRunnerFactory.
func ExtismRunnerFactoryWithTimeout(hf *HostFunctions, timeoutMs int) WASMRunnerFactory {
	return func(slug string, wasmBytes []byte, config map[string]string, limits ManifestLimits) (WASMRunner, error) {
		manifest := extism.Manifest{
			Wasm: []extism.Wasm{
				extism.WasmData{Data: wasmBytes},
			},
		}

		applyManifestLimits(&manifest, config, limits)

		// Explicit timeout overrides the fuel-based timeout set by
		// applyManifestLimits.
		if timeoutMs > 0 {
			manifest.Timeout = uint64(timeoutMs)
		}

		pluginConfig := extism.PluginConfig{
			EnableWasi: true,
		}

		// Build host functions bound to this plugin slug.
		hostFns := buildExtismHostFunctions(hf, slug)

		p, err := extism.NewPlugin(context.Background(), manifest, pluginConfig, hostFns)
		if err != nil {
			return nil, fmt.Errorf("create extism plugin with timeout: %w", err)
		}

		return &ExtismRunner{plugin: p}, nil
	}
}

// logRequest is the JSON structure WASM guests send when calling the log host
// function. Level must be one of: debug, info, warn, error.
type logRequest struct {
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// hostFunctionName constants for host functions exposed to WASM guests.
const hostFunctionNameLog = "log"

// maxLogPayloadBytes is the maximum size of a log payload from a WASM guest.
// Oversized entries are silently dropped to prevent a misbehaving plugin from
// flooding the host with unbounded log data.
const maxLogPayloadBytes = 64 << 10 // 64 KB

// buildExtismHostFunctions creates the Extism host function definitions bound
// to a specific plugin slug. If hf is nil, no host functions are registered
// (the WASM guest cannot call back into the host).
func buildExtismHostFunctions(hf *HostFunctions, slug string) []extism.HostFunction {
	if hf == nil {
		return nil
	}

	logFn := extism.NewHostFunctionWithStack(
		hostFunctionNameLog,
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			offset := stack[0]
			input, err := p.ReadBytes(offset)
			if err != nil {
				return
			}
			if len(input) > maxLogPayloadBytes {
				return // silently drop oversized log entries
			}

			var req logRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return
			}

			hf.Log(slug, req.Level, req.Message, req.Fields)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		nil,
	)

	return []extism.HostFunction{logFn}
}

// Call invokes an exported WASM function by name. Input and output bytes use
// the Extism PDK memory model (not stdin/stdout).
func (r *ExtismRunner) Call(ctx context.Context, funcName string, input []byte) ([]byte, error) {
	exit, output, err := r.plugin.CallWithContext(ctx, funcName, input)
	if err != nil {
		return nil, fmt.Errorf("extism call %q: %w", funcName, err)
	}

	const exitOK = 0
	if exit != exitOK {
		return nil, fmt.Errorf("extism call %q: non-zero exit code %d", funcName, exit)
	}
	return output, nil
}

// Close releases all resources held by the underlying Extism plugin and the
// wazero runtime.
func (r *ExtismRunner) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	return r.plugin.Close(ctx)
}

// closeTimeout limits how long we wait for the wazero runtime to shut down.
const closeTimeout = 5 * time.Second

// WASMPagesPerMB is the conversion factor from megabytes to WASM pages
// (1 WASM page = 64 KB, so 1 MB = 16 pages). Exported for use in runtime
// configurations that need to translate ManifestLimits.MaxMemoryMB.
const WASMPagesPerMB = 16
