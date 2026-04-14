package hookfn_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch/hookdispatchtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookfn"
)

// testReq is a test request payload.
type testReq struct {
	UserID string `json:"user_id"`
	PlanID string `json:"plan_id"`
}

// testResp is a test response payload.
type testResp struct {
	Block  bool   `json:"block"`
	Reason string `json:"reason,omitempty"`
}

const testHookName = "test.hook"

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestNewSyncSafe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(*hookdispatchtest.MockDispatcher)
		wantResp *testResp
		wantErr  bool
	}{
		{
			name: "successful dispatch and unmarshal",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				respBytes, _ := json.Marshal(testResp{Block: true, Reason: "geo-blocked"})
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Payload: respBytes})
			},
			wantResp: &testResp{Block: true, Reason: "geo-blocked"},
			wantErr:  false,
		},
		{
			name: "dispatch error returns nil nil",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Err: errors.New("plugin crashed")})
			},
			wantResp: nil,
			wantErr:  false,
		},
		{
			name: "nil payload returns nil nil",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Payload: nil})
			},
			wantResp: nil,
			wantErr:  false,
		},
		{
			name: "invalid JSON response returns nil nil",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Payload: json.RawMessage(`{invalid`)})
			},
			wantResp: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &hookdispatchtest.MockDispatcher{}
			tt.setup(d)

			fn := hookfn.NewSyncSafe[testReq, testResp](d, testHookName, discardLogger())

			resp, err := fn(context.Background(), testReq{UserID: "u1", PlanID: "p1"})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.wantResp == nil {
				assert.Nil(t, resp)
			} else {
				require.NotNil(t, resp)
				assert.Equal(t, tt.wantResp.Block, resp.Block)
				assert.Equal(t, tt.wantResp.Reason, resp.Reason)
			}

			d.AssertExpectations(t)
		})
	}
}

func TestNewSync(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(*hookdispatchtest.MockDispatcher)
		wantResp *testResp
		wantErr  bool
	}{
		{
			name: "successful dispatch and unmarshal",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				respBytes, _ := json.Marshal(testResp{Block: false})
				d.On("DispatchSync", mock.Anything, testHookName, mock.Anything).
					Return(json.RawMessage(respBytes), nil)
			},
			wantResp: &testResp{Block: false},
			wantErr:  false,
		},
		{
			name: "dispatch error propagated",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSync", mock.Anything, testHookName, mock.Anything).
					Return(nil, errors.New("no plugin registered"))
			},
			wantResp: nil,
			wantErr:  true,
		},
		{
			name: "unmarshal error propagated",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSync", mock.Anything, testHookName, mock.Anything).
					Return(json.RawMessage(`{not-json`), nil)
			},
			wantResp: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &hookdispatchtest.MockDispatcher{}
			tt.setup(d)

			fn := hookfn.NewSync[testReq, testResp](d, testHookName)

			resp, err := fn(context.Background(), testReq{UserID: "u1", PlanID: "p1"})

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.wantResp.Block, resp.Block)
			}

			d.AssertExpectations(t)
		})
	}
}

func TestNewSyncFireAndForget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*hookdispatchtest.MockDispatcher)
		wantErr bool
	}{
		{
			name: "successful dispatch",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSync", mock.Anything, testHookName, mock.Anything).
					Return(json.RawMessage(`{}`), nil)
			},
			wantErr: false,
		},
		{
			name: "dispatch error propagated",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSync", mock.Anything, testHookName, mock.Anything).
					Return(nil, errors.New("refund failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &hookdispatchtest.MockDispatcher{}
			tt.setup(d)

			fn := hookfn.NewSyncFireAndForget[testReq](d, testHookName)

			err := fn(context.Background(), testReq{UserID: "u1", PlanID: "p1"})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			d.AssertExpectations(t)
		})
	}
}

func TestNewAsync(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*hookdispatchtest.MockDispatcher)
		payload any
	}{
		{
			name: "successful async dispatch",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchAsync", mock.Anything, testHookName, mock.Anything).Return()
			},
			payload: testReq{UserID: "u1", PlanID: "p1"},
		},
		{
			name: "marshal error logged silently",
			setup: func(_ *hookdispatchtest.MockDispatcher) {
				// DispatchAsync should NOT be called when marshal fails.
			},
			payload: make(chan int), // channels are not JSON-serializable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &hookdispatchtest.MockDispatcher{}
			tt.setup(d)

			fn := hookfn.NewAsync(d, discardLogger())

			// Should not panic regardless of payload.
			fn(context.Background(), testHookName, tt.payload)

			d.AssertExpectations(t)
		})
	}
}

func TestNewSyncRaw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(*hookdispatchtest.MockDispatcher)
		wantResp []byte
		wantErr  bool
	}{
		{
			name: "successful dispatch returns raw bytes",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Payload: json.RawMessage(`{"key":"value"}`)})
			},
			wantResp: []byte(`{"key":"value"}`),
			wantErr:  false,
		},
		{
			name: "dispatch error returns nil nil",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Err: errors.New("plugin failed")})
			},
			wantResp: nil,
			wantErr:  false,
		},
		{
			name: "nil payload returns nil nil",
			setup: func(d *hookdispatchtest.MockDispatcher) {
				d.On("DispatchSyncSafe", mock.Anything, testHookName, mock.Anything).
					Return(&hookdispatch.ChainResult{Payload: nil})
			},
			wantResp: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &hookdispatchtest.MockDispatcher{}
			tt.setup(d)

			fn := hookfn.NewSyncRaw(d, discardLogger())

			resp, err := fn(context.Background(), testHookName, testReq{UserID: "u1"})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantResp, resp)
			d.AssertExpectations(t)
		})
	}
}
