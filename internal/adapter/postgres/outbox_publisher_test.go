package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

func TestOutboxPublisherImplementsPublisher(t *testing.T) {
	// Compile-time check is in the production code via var _ domainevent.Publisher.
	// This test validates the assertion at the type level from the test package.
	var pub domainevent.Publisher = postgres.NewOutboxPublisher(nil)
	assert.NotNil(t, pub)
}
