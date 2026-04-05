// Package hookdispatchtest provides mock implementations of hookdispatch.Dispatcher
// for use in domain-level unit tests. This package ensures domain tests never
// import internal/plugin.
package hookdispatchtest

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/mock"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// MockDispatcher is a testify/mock implementation of hookdispatch.Dispatcher.
type MockDispatcher struct {
	mock.Mock
}

// DispatchSync dispatches a hook synchronously, returning the mocked response.
func (m *MockDispatcher) DispatchSync(ctx context.Context, hookName string, payload json.RawMessage) (json.RawMessage, error) {
	args := m.Called(ctx, hookName, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}

// DispatchSyncVersioned dispatches a versioned hook synchronously, returning
// the mocked response.
func (m *MockDispatcher) DispatchSyncVersioned(ctx context.Context, hookName string, currentVersion int, payload json.RawMessage) (json.RawMessage, error) {
	args := m.Called(ctx, hookName, currentVersion, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}

// DispatchSyncSafe dispatches a compensating hook chain synchronously,
// returning the mocked ChainResult.
func (m *MockDispatcher) DispatchSyncSafe(ctx context.Context, hookName string, payload json.RawMessage) *hookdispatch.ChainResult {
	args := m.Called(ctx, hookName, payload)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*hookdispatch.ChainResult)
}

// BeginFlow records the call and returns the context unchanged. Domain tests do
// not need real flow bindings since they mock the dispatcher.
func (m *MockDispatcher) BeginFlow(ctx context.Context) context.Context {
	m.Called(ctx)
	return ctx
}
