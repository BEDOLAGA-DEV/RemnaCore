package telegram

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestDisplayNameFor(t *testing.T) {
	assert.Equal(t, "First Last", displayNameFor(&models.User{FirstName: "First", LastName: "Last"}))
	assert.Equal(t, "First", displayNameFor(&models.User{FirstName: "First"}))
	assert.Equal(t, "neo", displayNameFor(&models.User{Username: "neo"}))
}
