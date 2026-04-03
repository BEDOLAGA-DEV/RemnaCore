// Package hookdispatchtest provides mock implementations of hookdispatch.Dispatcher
// for use in domain-level unit tests. This package ensures domain tests never
// import internal/plugin.
package hookdispatchtest

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/mock"
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
