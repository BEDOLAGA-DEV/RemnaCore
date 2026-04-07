package plugin

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginHealthProbe_ProbeAll_Healthy(t *testing.T) {
	var calls atomic.Int64
	factory := mockFactory(func(_ context.Context, funcName string, _ []byte) ([]byte, error) {
		if funcName == healthProbeFuncName {
			calls.Add(1)
		}
		return nil, nil
	})

	rp := NewRuntimePool(testErrorLogger(), factory)
	p := testPlugin("healthy-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	probe := NewPluginHealthProbe(rp, testErrorLogger())
	probe.probeAll(context.Background())

	assert.Equal(t, int64(1), calls.Load(), "health probe should call __health_check once per plugin")
}

func TestPluginHealthProbe_ProbeAll_Unhealthy(t *testing.T) {
	factory := mockFactory(func(_ context.Context, funcName string, _ []byte) ([]byte, error) {
		if funcName == healthProbeFuncName {
			return nil, fmt.Errorf("function not found: %s", healthProbeFuncName)
		}
		return nil, nil
	})

	rp := NewRuntimePool(testErrorLogger(), factory)
	p := testPlugin("missing-health-check")
	require.NoError(t, rp.LoadPlugin(p))

	probe := NewPluginHealthProbe(rp, testErrorLogger())

	// Should not panic; probe is best-effort.
	probe.probeAll(context.Background())
}

func TestPluginHealthProbe_ProbeAll_NoPlugins(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))
	probe := NewPluginHealthProbe(rp, testErrorLogger())

	// Should be a no-op without error.
	probe.probeAll(context.Background())
}

func TestPluginHealthProbe_ProbeAll_MultiplePlugins(t *testing.T) {
	var calls atomic.Int64
	factory := mockFactory(func(_ context.Context, funcName string, _ []byte) ([]byte, error) {
		if funcName == healthProbeFuncName {
			calls.Add(1)
		}
		return nil, nil
	})

	rp := NewRuntimePool(testErrorLogger(), factory)

	p1 := testPlugin("plugin-a")
	require.NoError(t, rp.LoadPlugin(p1))

	p2 := testPlugin("plugin-b")
	require.NoError(t, rp.LoadPlugin(p2))

	probe := NewPluginHealthProbe(rp, testErrorLogger())
	probe.probeAll(context.Background())

	assert.Equal(t, int64(2), calls.Load(), "health probe should check all loaded plugins")
}

func TestPluginHealthProbe_Run_CancelsOnContext(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))
	probe := NewPluginHealthProbe(rp, testErrorLogger())
	// Use a very short interval so the test doesn't take long.
	probe.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		probe.Run(ctx)
		close(done)
	}()

	// Let it tick a few times.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Run exited after context cancellation.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestPluginHealthProbe_ProbeAll_RespectsTimeout(t *testing.T) {
	// Simulate a slow health check that exceeds the probe timeout.
	factory := mockFactory(func(ctx context.Context, funcName string, _ []byte) ([]byte, error) {
		if funcName == healthProbeFuncName {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(10 * time.Second):
				return nil, nil
			}
		}
		return nil, nil
	})

	rp := NewRuntimePool(testErrorLogger(), factory)
	p := testPlugin("slow-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	probe := NewPluginHealthProbe(rp, testErrorLogger())

	// probeAll uses healthProbeTimeout (5s), but we want the test to be fast.
	// The probe creates a context.WithTimeout internally, so we set a parent
	// deadline that is shorter.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	probe.probeAll(ctx)
	elapsed := time.Since(start)

	// Should complete well before the 10s mock sleep.
	assert.Less(t, elapsed, 2*time.Second, "probe should respect context timeout")
}
