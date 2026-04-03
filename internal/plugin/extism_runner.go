package plugin

import (
	"context"
	"fmt"
	"time"

	extism "github.com/extism/go-sdk"
)

// ExtismRunner implements WASMRunner using the Extism Go SDK backed by wazero.
type ExtismRunner struct {
	plugin *extism.Plugin
}

// ExtismRunnerFactory creates a WASMRunnerFactory that produces ExtismRunners
// with WASI support. The returned runners use wazero as the underlying runtime.
func ExtismRunnerFactory() WASMRunnerFactory {
	return func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		manifest := extism.Manifest{
			Wasm: []extism.Wasm{
				extism.WasmData{Data: wasmBytes},
			},
		}

		// Pass plugin config as Extism manifest config so plugins can
		// retrieve values via pdk.ConfigGet.
		if len(config) > 0 {
			manifest.Config = make(map[string]string, len(config))
			for k, v := range config {
				manifest.Config[k] = v
			}
		}

		pluginConfig := extism.PluginConfig{
			EnableWasi: true,
		}

		p, err := extism.NewPlugin(context.Background(), manifest, pluginConfig, nil)
		if err != nil {
			return nil, fmt.Errorf("create extism plugin: %w", err)
		}

		return &ExtismRunner{plugin: p}, nil
	}
}

// ExtismRunnerFactoryWithTimeout creates a WASMRunnerFactory that applies per-
// plugin timeout and memory limits from the effective manifest limits.
func ExtismRunnerFactoryWithTimeout(timeoutMs int) WASMRunnerFactory {
	return func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		manifest := extism.Manifest{
			Wasm: []extism.Wasm{
				extism.WasmData{Data: wasmBytes},
			},
		}

		if timeoutMs > 0 {
			manifest.Timeout = uint64(timeoutMs)
		}

		if len(config) > 0 {
			manifest.Config = make(map[string]string, len(config))
			for k, v := range config {
				manifest.Config[k] = v
			}
		}

		pluginConfig := extism.PluginConfig{
			EnableWasi: true,
		}

		p, err := extism.NewPlugin(context.Background(), manifest, pluginConfig, nil)
		if err != nil {
			return nil, fmt.Errorf("create extism plugin with timeout: %w", err)
		}

		return &ExtismRunner{plugin: p}, nil
	}
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
