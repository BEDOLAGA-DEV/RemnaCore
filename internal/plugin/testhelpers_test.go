package plugin

import (
	"context"
	"log/slog"
	"os"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// testPublisher is a mock event publisher shared across plugin tests.
type testPublisher struct {
	events []domainevent.Event
}

func (p *testPublisher) Publish(_ context.Context, event domainevent.Event) error {
	p.events = append(p.events, event)
	return nil
}

// testErrorLogger returns a logger that only emits error-level messages,
// keeping test output clean.
func testErrorLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
