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
//
// PublishBatch delegates to individual Publish calls so that existing test
// expectations on per-event Publish continue to work transparently after the
// PublishAll refactor (which now calls PublishBatch instead of looping Publish).
type MockPublisher struct {
	mock.Mock
}

// Publish records the call and returns the configured error.
func (m *MockPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// PublishBatch fans out to individual Publish calls so that existing per-event
// mock expectations remain valid. This mirrors the production behavior where
// PublishAll was previously a loop over Publish.
func (m *MockPublisher) PublishBatch(ctx context.Context, events []domainevent.Event) error {
	for _, event := range events {
		if err := m.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Ensure MockPublisher satisfies domainevent.Publisher at compile time.
var _ domainevent.Publisher = (*MockPublisher)(nil)
