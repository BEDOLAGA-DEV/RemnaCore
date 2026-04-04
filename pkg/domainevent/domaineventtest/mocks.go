// Package domaineventtest provides shared mock implementations of
// domainevent interfaces for use in unit tests across all bounded contexts.
package domaineventtest

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// MockPublisher is a testify/mock implementation of domainevent.Publisher.
// Use this instead of defining per-context duplicates.
type MockPublisher struct {
	mock.Mock
}

// Publish records the call and returns the configured error.
func (m *MockPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// Ensure MockPublisher satisfies domainevent.Publisher at compile time.
var _ domainevent.Publisher = (*MockPublisher)(nil)
