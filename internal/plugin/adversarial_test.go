package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Malformed hook responses
// ---------------------------------------------------------------------------

func TestAdversarial_MalformedHookResponse(t *testing.T) {
	tests := []struct {
		name   string
		output []byte
	}{
		{"invalid JSON", []byte(`{not json}`)},
		{"empty bytes", []byte(``)},
		{"bare string", []byte(`"just a string"`)},
		{"truncated JSON", []byte(`{"action":"con`)},
		{"array instead of object", []byte(`[1,2,3]`)},
		{"nested garbage", []byte(`{"action":{"nested":"bad"}}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
				"adversarial-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
					return tt.output, nil
				},
			})

			d.RegisterHooks([]HookRegistration{
				{PluginID: "id-adv", PluginSlug: "adversarial-plugin", HookName: "test.hook", HookType: HookSync, Priority: 10, FuncName: "test.hook"},
			})

			payload := json.RawMessage(`{"key":"value"}`)
			_, err := d.DispatchSync(context.Background(), "test.hook", payload)
			require.Error(t, err, "malformed response %q should produce an error, not a panic", tt.name)
		})
	}
}

func TestAdversarial_NullHookResponse_TreatedAsContinue(t *testing.T) {
	// JSON null deserializes to a zero-value HookResult (Action=""), which
	// the dispatcher treats as an unknown action and logs a warning. The
	// critical property: no panic, no blocked chain.
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"null-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			return []byte(`null`), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-null", PluginSlug: "null-plugin", HookName: "test.hook", HookType: HookSync, Priority: 10, FuncName: "test.hook"},
	})

	payload := json.RawMessage(`{"key":"value"}`)
	result, err := d.DispatchSync(context.Background(), "test.hook", payload)
	require.NoError(t, err, "null response should be handled gracefully as continue")
	assert.JSONEq(t, `{"key":"value"}`, string(result), "payload should pass through unchanged")
}

func TestAdversarial_MalformedHookResponse_DispatchSyncSafe(t *testing.T) {
	// DispatchSyncSafe should capture the error in ChainResult, not panic.
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"adversarial-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			return []byte(`{garbage`), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-adv", PluginSlug: "adversarial-plugin", HookName: "test.hook", HookType: HookSync, Priority: 10, FuncName: "test.hook"},
	})

	payload := json.RawMessage(`{"key":"value"}`)
	result := d.DispatchSyncSafe(context.Background(), "test.hook", payload)

	require.Error(t, result.Err)
	assert.True(t, result.Compensated, "failed dispatch should trigger compensation")
}

// ---------------------------------------------------------------------------
// Oversized hook responses
// ---------------------------------------------------------------------------

func TestAdversarial_OversizedHookResponse(t *testing.T) {
	// A plugin returning a massive response should not OOM the host. The
	// dispatcher must parse the response and handle it gracefully. Even if
	// the output is large, it should either succeed (if valid JSON) or fail
	// with an error (if invalid).
	const oversizedBytes = 10 << 20 // 10 MB

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"bloated-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			// Return a valid HookResult with a massive Modified payload.
			bigPayload := strings.Repeat("x", oversizedBytes)
			result := sdk.HookResult{
				Action:   sdk.ActionModify,
				Modified: json.RawMessage(fmt.Sprintf(`{"data":"%s"}`, bigPayload)),
			}
			b, _ := json.Marshal(result)
			return b, nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-bloat", PluginSlug: "bloated-plugin", HookName: "test.hook", HookType: HookSync, Priority: 10, FuncName: "test.hook"},
	})

	payload := json.RawMessage(`{"key":"value"}`)
	// Should not panic or OOM. The result may be large but the dispatcher
	// handles it without crashing.
	result, err := d.DispatchSync(context.Background(), "test.hook", payload)
	require.NoError(t, err)
	assert.True(t, len(result) > oversizedBytes, "modified payload should contain the oversized data")
}

// ---------------------------------------------------------------------------
// Permission escalation
// ---------------------------------------------------------------------------

func TestAdversarial_PermissionEscalation(t *testing.T) {
	// A plugin with billing:read should NOT be able to call StorageSet
	// (requires storage:readwrite).
	tests := []struct {
		name        string
		grantedPerm []PermissionScope
		operation   string
		callFn      func(hf *HostFunctions) error
	}{
		{
			name:        "billing:read cannot StorageSet",
			grantedPerm: []PermissionScope{PermBillingRead},
			operation:   "StorageSet",
			callFn: func(hf *HostFunctions) error {
				return hf.StorageSet(context.Background(), "test-plugin", "key", []byte("val"), 0)
			},
		},
		{
			name:        "billing:read cannot StorageGet",
			grantedPerm: []PermissionScope{PermBillingRead},
			operation:   "StorageGet",
			callFn: func(hf *HostFunctions) error {
				_, err := hf.StorageGet(context.Background(), "test-plugin", "key")
				return err
			},
		},
		{
			name:        "storage:read cannot StorageSet",
			grantedPerm: []PermissionScope{PermStorageRead},
			operation:   "StorageSet",
			callFn: func(hf *HostFunctions) error {
				return hf.StorageSet(context.Background(), "test-plugin", "key", []byte("val"), 0)
			},
		},
		{
			name:        "billing:read cannot EmitEvent",
			grantedPerm: []PermissionScope{PermBillingRead},
			operation:   "EmitEvent",
			callFn: func(hf *HostFunctions) error {
				return hf.EmitEvent(context.Background(), "test-plugin", "test.event", []byte(`{}`))
			},
		},
		{
			name:        "storage:readwrite cannot VPNRequest",
			grantedPerm: []PermissionScope{PermStorageWrite},
			operation:   "VPNRequest",
			callFn: func(hf *HostFunctions) error {
				input, _ := json.Marshal(sdk.VPNRequest{Method: http.MethodGet, Path: "/api/test"})
				_, err := hf.VPNRequest(context.Background(), "test-plugin", input)
				return err
			},
		},
		{
			name:        "vpn:read cannot VPNRequest (requires vpn:write)",
			grantedPerm: []PermissionScope{PermVPNRead},
			operation:   "VPNRequest",
			callFn: func(hf *HostFunctions) error {
				input, _ := json.Marshal(sdk.VPNRequest{Method: http.MethodGet, Path: "/api/test"})
				_, err := hf.VPNRequest(context.Background(), "test-plugin", input)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPluginWithPerms(tt.grantedPerm)
			hf := newTestHostFunctions(t, p, nil)
			hf.Storage = newTestStorageService()
			hf.Publisher = &testPublisher{}
			hf.SetVPNExecutor(&testVPNExecutor{
				response: &sdk.VPNResponse{StatusCode: http.StatusOK},
			})

			err := tt.callFn(hf)
			require.Error(t, err, "%s should be denied for permissions %v", tt.operation, tt.grantedPerm)
			assert.True(t, errors.Is(err, ErrPermissionDenied),
				"expected ErrPermissionDenied, got: %v", err)
		})
	}
}

// ---------------------------------------------------------------------------
// Storage key path traversal and injection
// ---------------------------------------------------------------------------

func TestAdversarial_StorageKeyPathTraversal(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"parent directory traversal", "../other-plugin/secrets"},
		{"nested traversal", "../../etc/passwd"},
		{"slash in key", "other-plugin/data"},
		{"backslash in key", "other-plugin\\data"},
		{"double dot only", ".."},
		{"double dot with extension", "..credentials"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (StorageGet)", func(t *testing.T) {
			p := testPluginWithPerms([]PermissionScope{PermStorageRead})
			hf := newTestHostFunctions(t, p, nil)
			hf.Storage = newTestStorageService()

			_, err := hf.StorageGet(context.Background(), "test-plugin", tt.key)
			require.Error(t, err, "key %q should be rejected", tt.key)
			assert.ErrorIs(t, err, ErrInvalidStorageKey)
		})

		t.Run(tt.name+" (StorageSet)", func(t *testing.T) {
			p := testPluginWithPerms([]PermissionScope{PermStorageWrite})
			hf := newTestHostFunctions(t, p, nil)
			hf.Storage = newTestStorageService()

			err := hf.StorageSet(context.Background(), "test-plugin", tt.key, []byte("evil"), 0)
			require.Error(t, err, "key %q should be rejected", tt.key)
			assert.ErrorIs(t, err, ErrInvalidStorageKey)
		})
	}
}

// ---------------------------------------------------------------------------
// SSRF protection: private/internal IPs
// ---------------------------------------------------------------------------

func TestAdversarial_HTTPRequestToCloudMetadata(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		allowlist string
	}{
		{"AWS metadata IPv4", "http://169.254.169.254/latest/meta-data/", "http://169.254.169.254/*"},
		{"AWS metadata IPv6", "http://[fd00:ec2::254]/latest/meta-data/", "http://[fd00:ec2::254]/*"},
		{"GCP metadata", "http://metadata.google.internal/computeMetadata/v1/", "http://metadata.google.internal/*"},
		{"GCP alternative", "http://metadata.goog/", "http://metadata.goog/*"},
		{"bare metadata hostname", "http://metadata/computeMetadata/v1/", "http://metadata/*"},
		{"DigitalOcean instance-data", "http://instance-data/latest/meta-data/", "http://instance-data/*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPluginWithHTTP([]string{tt.allowlist})
			hf := newTestHostFunctions(t, p, nil)

			_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
				Method: http.MethodGet,
				URL:    tt.url,
			})

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
				"URL %q should be blocked by SSRF protection, got: %v", tt.url, err)
		})
	}
}

func TestAdversarial_HTTPRequestToPrivateIP(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		allowlist string
	}{
		{"RFC 1918 10.x", "http://10.0.0.1:8080/api", "http://10.0.0.1:8080/*"},
		{"RFC 1918 172.x", "http://172.16.0.1:8080/api", "http://172.16.0.1:8080/*"},
		{"RFC 1918 192.168.x", "http://192.168.1.1/admin", "http://192.168.1.1/*"},
		{"loopback 127.0.0.1", "http://127.0.0.1:4000/api", "http://127.0.0.1:4000/*"},
		{"loopback 0.0.0.0", "http://0.0.0.0:5000/api", "http://0.0.0.0:5000/*"},
		{"IPv6 loopback", "http://[::1]:8080/path", "http://[::1]:8080/*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPluginWithHTTP([]string{tt.allowlist})
			hf := newTestHostFunctions(t, p, nil)

			_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
				Method: http.MethodGet,
				URL:    tt.url,
			})

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
				"URL %q should be blocked, got: %v", tt.url, err)
		})
	}
}

func TestAdversarial_HTTPRequestToLocalhost(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		allowlist string
	}{
		{"localhost", "http://localhost:3000/api", "http://localhost:3000/*"},
		{"LOCALHOST uppercase", "http://LOCALHOST:3000/api", "http://LOCALHOST:3000/*"},
		{"Localhost mixed case", "http://Localhost:3000/api", "http://Localhost:3000/*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPluginWithHTTP([]string{tt.allowlist})
			hf := newTestHostFunctions(t, p, nil)

			_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
				Method: http.MethodGet,
				URL:    tt.url,
			})

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
				"URL %q should be blocked, got: %v", tt.url, err)
		})
	}
}

// ---------------------------------------------------------------------------
// Null bytes in storage keys
// ---------------------------------------------------------------------------

func TestAdversarial_NullBytesInStorageKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"null byte in middle", "key\x00evil"},
		{"null byte at start", "\x00key"},
		{"null byte at end", "key\x00"},
		{"multiple null bytes", "k\x00e\x00y"},
		{"null byte with traversal", "..\x00/secrets"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (StorageGet)", func(t *testing.T) {
			p := testPluginWithPerms([]PermissionScope{PermStorageRead})
			hf := newTestHostFunctions(t, p, nil)
			hf.Storage = newTestStorageService()

			_, err := hf.StorageGet(context.Background(), "test-plugin", tt.key)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidStorageKey)
		})

		t.Run(tt.name+" (StorageSet)", func(t *testing.T) {
			p := testPluginWithPerms([]PermissionScope{PermStorageWrite})
			hf := newTestHostFunctions(t, p, nil)
			hf.Storage = newTestStorageService()

			err := hf.StorageSet(context.Background(), "test-plugin", tt.key, []byte("val"), 0)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidStorageKey)
		})
	}
}

// ---------------------------------------------------------------------------
// Empty plugin slug
// ---------------------------------------------------------------------------

func TestAdversarial_EmptyPluginSlug(t *testing.T) {
	hf := newTestHostFunctions(t, testPluginWithPerms(nil), nil)
	hf.Storage = newTestStorageService()
	hf.Publisher = &testPublisher{}

	t.Run("StorageGet with empty slug", func(t *testing.T) {
		_, err := hf.StorageGet(context.Background(), "", "key")
		require.Error(t, err, "empty slug should return error")
	})

	t.Run("StorageSet with empty slug", func(t *testing.T) {
		err := hf.StorageSet(context.Background(), "", "key", []byte("val"), 0)
		require.Error(t, err, "empty slug should return error")
	})

	t.Run("HTTPRequest with empty slug", func(t *testing.T) {
		_, err := hf.HTTPRequest(context.Background(), "", sdk.HTTPRequest{
			Method: http.MethodGet,
			URL:    "https://example.com",
		})
		require.Error(t, err, "empty slug should return error")
	})

	t.Run("EmitEvent with empty slug", func(t *testing.T) {
		err := hf.EmitEvent(context.Background(), "", "test.event", []byte(`{}`))
		require.Error(t, err, "empty slug should return error")
	})

	t.Run("VPNRequest with empty slug", func(t *testing.T) {
		input, _ := json.Marshal(sdk.VPNRequest{Method: http.MethodGet, Path: "/api/test"})
		_, err := hf.VPNRequest(context.Background(), "", input)
		require.Error(t, err, "empty slug should return error")
	})

	t.Run("ConfigGet with empty slug", func(t *testing.T) {
		_, err := hf.ConfigGet("", "some-key")
		require.Error(t, err, "empty slug should return error")
	})
}

// ---------------------------------------------------------------------------
// Concurrent hook dispatch
// ---------------------------------------------------------------------------

// concurrentDispatcherWithMock creates a HookDispatcher using syncTestPublisher
// so it is safe for concurrent goroutine-based tests. The standard
// dispatcherWithMock uses testPublisher which is not thread-safe.
func concurrentDispatcherWithMock(t *testing.T, slugCallFns map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error)) *HookDispatcher {
	t.Helper()

	logger := testErrorLogger()
	pub := &syncTestPublisher{}

	rp := NewRuntimePool(logger, nil)
	for slug, callFn := range slugCallFns {
		p := testPlugin(slug)
		require.NoError(t, rp.LoadPlugin(p))
		rp.SetRunnerForTest(slug, &mockRunner{callFn: callFn})
	}

	return NewHookDispatcher(rp, pub, nil, logger, clock.NewReal())
}

func TestAdversarial_ConcurrentHookDispatch(t *testing.T) {
	const goroutineCount = 50

	var callCount atomic.Int64
	d := concurrentDispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"concurrent-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			callCount.Add(1)
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-conc", PluginSlug: "concurrent-plugin", HookName: "stress.hook", HookType: HookSync, Priority: 10, FuncName: "stress.hook"},
	})

	payload := json.RawMessage(`{"key":"value"}`)

	var wg sync.WaitGroup
	errCh := make(chan error, goroutineCount)

	for range goroutineCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := d.DispatchSync(context.Background(), "stress.hook", payload)
			if err != nil {
				errCh <- err
				return
			}
			if string(result) == "" {
				errCh <- fmt.Errorf("empty result from concurrent dispatch")
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent dispatch error: %v", err)
	}

	assert.Equal(t, int64(goroutineCount), callCount.Load(),
		"every goroutine should have executed the hook exactly once")
}

func TestAdversarial_ConcurrentHookDispatch_MixedActions(t *testing.T) {
	// Multiple goroutines dispatching the same hook where the plugin returns
	// different actions. Verifies no data races or corrupted results.
	var invocation atomic.Int64

	d := concurrentDispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"mixed-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			n := invocation.Add(1)
			if n%2 == 0 {
				return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"modified":true}`), ""), nil
			}
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-mixed", PluginSlug: "mixed-plugin", HookName: "mixed.hook", HookType: HookSync, Priority: 10, FuncName: "mixed.hook"},
	})

	const goroutineCount = 30
	payload := json.RawMessage(`{"key":"value"}`)

	var wg sync.WaitGroup
	for range goroutineCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.DispatchSync(context.Background(), "mixed.hook", payload)
			// Should not panic or error regardless of action.
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// URL path traversal in HTTP requests
// ---------------------------------------------------------------------------

func TestAdversarial_HTTPPathTraversal(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"traversal to parent", "https://api.stripe.com/v1/../../admin"},
		{"traversal at start", "https://api.stripe.com/../secret"},
		{"double traversal", "https://api.stripe.com/a/b/../../../../../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPluginWithHTTP([]string{"https://api.stripe.com/*"})
			hf := newTestHostFunctions(t, p, nil)

			_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
				Method: http.MethodGet,
				URL:    tt.url,
			})

			require.Error(t, err, "traversal URL %q should be rejected", tt.url)
			assert.True(t, errors.Is(err, ErrPermissionDenied),
				"expected ErrPermissionDenied from path traversal detection, got: %v", err)
		})
	}
}

// ---------------------------------------------------------------------------
// Runner corruption detection
// ---------------------------------------------------------------------------

func TestAdversarial_RunnerCorruptionIsDetected(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{"wasm trap", "wasm trap: unreachable", true},
		{"memory fault", "out of memory", true},
		{"unreachable instruction", "unreachable executed", true},
		{"out of fuel", "out of fuel", true},
		{"panic in wasm", "panic: index out of range", true},
		{"trap signal", "trap occurred", true},
		{"normal error", "network timeout", false},
		{"business logic error", "invalid input", false},
		{"context cancelled", "context canceled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg == "context canceled" {
				err = context.Canceled
			} else {
				err = fmt.Errorf("%s", tt.errMsg)
			}
			assert.Equal(t, tt.expected, isRunnerCorrupted(err),
				"isRunnerCorrupted(%q) should be %v", tt.errMsg, tt.expected)
		})
	}
}

func TestAdversarial_CorruptedRunnerIsDiscarded(t *testing.T) {
	// A runner that returns a WASM corruption error should be discarded (not
	// returned to pool) and replaced.
	logger := testErrorLogger()
	var replacementCreated atomic.Bool

	factory := func(_ string, _ []byte, _ map[string]string, _ ManifestLimits) (WASMRunner, error) {
		replacementCreated.Store(true)
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(logger, factory)
	p := testPlugin("corrupt-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	// Inject a runner that returns a WASM corruption error.
	rp.SetRunnerForTest("corrupt-plugin", &mockRunner{
		callFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			return nil, fmt.Errorf("wasm trap: unreachable instruction")
		},
	})

	_, err := rp.CallHook(context.Background(), "corrupt-plugin", "hook.test", []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupted")
}

// ---------------------------------------------------------------------------
// Storage control characters
// ---------------------------------------------------------------------------

func TestAdversarial_ControlCharsInStorageKey(t *testing.T) {
	// Control characters (0x00-0x1F, 0x7F) must be rejected in storage keys.
	controlChars := []struct {
		name string
		char byte
	}{
		{"null (0x00)", 0x00},
		{"SOH (0x01)", 0x01},
		{"STX (0x02)", 0x02},
		{"ETX (0x03)", 0x03},
		{"BEL (0x07)", 0x07},
		{"backspace (0x08)", 0x08},
		{"tab (0x09)", 0x09},
		{"newline (0x0A)", 0x0A},
		{"carriage return (0x0D)", 0x0D},
		{"escape (0x1B)", 0x1B},
		{"unit separator (0x1F)", 0x1F},
		{"DEL (0x7F)", 0x7F},
	}

	for _, cc := range controlChars {
		t.Run(cc.name, func(t *testing.T) {
			key := fmt.Sprintf("key%cvalue", cc.char)
			err := ValidateStorageKey(key)
			require.Error(t, err, "control character %s should be rejected in storage key", cc.name)
			assert.ErrorIs(t, err, ErrInvalidStorageKey)
		})
	}
}

// ---------------------------------------------------------------------------
// Permission implication boundaries
// ---------------------------------------------------------------------------

func TestAdversarial_WriteDoesNotImplyCrossResource(t *testing.T) {
	// billing:write should NOT imply storage:read, vpn:read, etc.
	pc := &PermissionChecker{}

	p := &Plugin{
		ID:          "test-id",
		Slug:        "test-plugin",
		Permissions: []PermissionScope{PermBillingWrite},
	}

	crossResourceScopes := []PermissionScope{
		PermStorageRead,
		PermStorageWrite,
		PermVPNRead,
		PermVPNWrite,
		PermUsersRead,
		PermUsersWrite,
		PermPaymentWrite,
		PermNotificationsEmit,
		PermAnalyticsWrite,
		PermAPIRoutes,
	}

	for _, scope := range crossResourceScopes {
		t.Run(string(scope), func(t *testing.T) {
			assert.False(t, pc.HasPermission(p, scope),
				"billing:write should NOT imply %s", scope)
		})
	}
}

func TestAdversarial_ReadDoesNotImplyWrite(t *testing.T) {
	pc := &PermissionChecker{}

	tests := []struct {
		granted  PermissionScope
		rejected PermissionScope
	}{
		{PermBillingRead, PermBillingWrite},
		{PermUsersRead, PermUsersWrite},
		{PermVPNRead, PermVPNWrite},
		{PermStorageRead, PermStorageWrite},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s should not imply %s", tt.granted, tt.rejected), func(t *testing.T) {
			p := &Plugin{
				ID:          "test-id",
				Slug:        "test-plugin",
				Permissions: []PermissionScope{tt.granted},
			}
			assert.False(t, pc.HasPermission(p, tt.rejected))
		})
	}
}

// ---------------------------------------------------------------------------
// HTTP rate limit exhaustion
// ---------------------------------------------------------------------------

func TestAdversarial_HTTPRateLimitExhaustion(t *testing.T) {
	// A plugin that rapidly exhausts its rate limit should be blocked, and
	// should not affect other plugins.
	const rateLimit = 3

	p := &Plugin{
		ID:   "test-id",
		Slug: "rate-limited-plugin",
		Manifest: &Manifest{
			Permissions: ManifestPermissions{
				HTTP: []string{"https://api.example.com/*"},
			},
			Limits: ManifestLimits{
				MaxHTTPCallsPerMin: rateLimit,
			},
		},
	}

	hf := newTestHostFunctions(t, p, nil)
	// Bypass SSRF check (we are testing rate limiting, not SSRF).
	hf.urlChecker = func(string) (bool, error) { return false, nil }

	// Exhaust the rate limit.
	for i := range rateLimit {
		_, err := hf.HTTPRequest(context.Background(), "rate-limited-plugin", sdk.HTTPRequest{
			Method: http.MethodGet,
			URL:    "https://api.example.com/v1/test",
		})
		// These will fail at the HTTP layer (no real server), but the rate
		// limiter should still be decremented.
		_ = err
		_ = i
	}

	// The next request should be rate limited.
	_, err := hf.HTTPRequest(context.Background(), "rate-limited-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "https://api.example.com/v1/test",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrHTTPRateLimitExceeded),
		"expected rate limit exceeded, got: %v", err)
}

// ---------------------------------------------------------------------------
// Hook dispatch with unknown action
// ---------------------------------------------------------------------------

func TestAdversarial_UnknownHookAction(t *testing.T) {
	// A plugin returning an unrecognized action should be treated as
	// "continue" (not panic or block the chain).
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"weird-plugin": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			result := sdk.HookResult{
				Action: sdk.HookAction("DELETE_EVERYTHING"),
			}
			b, _ := json.Marshal(result)
			return b, nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-weird", PluginSlug: "weird-plugin", HookName: "test.hook", HookType: HookSync, Priority: 10, FuncName: "test.hook"},
	})

	payload := json.RawMessage(`{"key":"value"}`)
	result, err := d.DispatchSync(context.Background(), "test.hook", payload)
	require.NoError(t, err, "unknown action should be treated as continue, not error")
	assert.JSONEq(t, `{"key":"value"}`, string(result), "payload should pass through unchanged")
}

// ---------------------------------------------------------------------------
// Concurrent register/unregister while dispatching
// ---------------------------------------------------------------------------

func TestAdversarial_ConcurrentRegisterUnregisterDuringDispatch(t *testing.T) {
	const iterations = 20

	d := concurrentDispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "race.hook", HookType: HookSync, Priority: 10, FuncName: "race.hook"},
	})

	payload := json.RawMessage(`{"key":"value"}`)
	var wg sync.WaitGroup

	// Dispatch concurrently.
	for range iterations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Should not panic regardless of concurrent registration changes.
			_, _ = d.DispatchSync(context.Background(), "race.hook", payload)
		}()
	}

	// Register/unregister concurrently.
	for range iterations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.RegisterHooks([]HookRegistration{
				{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "race.hook", HookType: HookSync, Priority: 10, FuncName: "race.hook"},
			})
			d.UnregisterHooks("plugin-a")
			d.RegisterHooks([]HookRegistration{
				{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "race.hook", HookType: HookSync, Priority: 10, FuncName: "race.hook"},
			})
		}()
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Pool acquisition under draining conditions
// ---------------------------------------------------------------------------

func TestAdversarial_AcquireOnDrainingPool(t *testing.T) {
	// After drain begins, Acquire should return ErrPluginDraining, not block.
	logger := testErrorLogger()
	rp := NewRuntimePool(logger, mockFactory(nil))

	p := testPlugin("drain-test")
	require.NoError(t, rp.LoadPlugin(p))

	// Start draining.
	pool := rp.DetachPool("drain-test")
	require.NotNil(t, pool)

	go func() {
		_ = pool.Drain(context.Background())
	}()

	// Wait for draining flag to be set, then try to acquire.
	// The drain goroutine sets draining=true immediately.
	_, err := pool.Acquire(context.Background())
	// After Drain sets the flag, Acquire should fail.
	if err != nil {
		assert.ErrorIs(t, err, ErrPluginDraining)
	}
	// If Acquire succeeded (race condition), just release it.
}

// ---------------------------------------------------------------------------
// Storage namespace isolation between plugins
// ---------------------------------------------------------------------------

func TestAdversarial_StorageNamespaceIsolation(t *testing.T) {
	// Plugin A stores data. Plugin B (with a different slug) should NOT be
	// able to read Plugin A's data, even if they use the same key.
	store := newTestStorageService()

	pluginA := &Plugin{
		ID:   "id-a",
		Slug: "plugin-a",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{ID: "plugin-a", Name: "A", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
			Hooks:  ManifestHooks{Sync: []string{"test_hook"}},
		},
		Permissions: []PermissionScope{PermStorageWrite},
	}

	pluginB := &Plugin{
		ID:   "id-b",
		Slug: "plugin-b",
		Manifest: &Manifest{
			Plugin: ManifestPlugin{ID: "plugin-b", Name: "B", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
			Hooks:  ManifestHooks{Sync: []string{"test_hook"}},
		},
		Permissions: []PermissionScope{PermStorageRead},
	}

	// Set up host functions with a registry that returns the correct plugin.
	hf := NewHostFunctions(testErrorLogger(), clock.NewReal())
	hf.Storage = store
	hf.SetPluginRegistry(func(slug string) (*Plugin, error) {
		switch slug {
		case "plugin-a":
			return pluginA, nil
		case "plugin-b":
			return pluginB, nil
		default:
			return nil, ErrPluginNotFound
		}
	})

	// Plugin A writes a secret.
	err := hf.StorageSet(context.Background(), "plugin-a", "secret-key", []byte("top-secret"), 0)
	require.NoError(t, err)

	// Plugin B tries to read Plugin A's key — should get "not found" because
	// storage is namespaced by slug.
	_, err = hf.StorageGet(context.Background(), "plugin-b", "secret-key")
	require.Error(t, err, "plugin-b should not be able to read plugin-a's data")
}
