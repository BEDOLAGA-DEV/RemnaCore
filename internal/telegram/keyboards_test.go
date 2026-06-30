package telegram

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestCabinetKeyboard_HTTPS_UsesWebAppButton(t *testing.T) {
	kb := CabinetKeyboard("https://shop.example.com/cabinet")
	assert.NotNil(t, kb)
	assert.Len(t, kb.InlineKeyboard, 1)
	assert.Len(t, kb.InlineKeyboard[0], 1)
	btn := kb.InlineKeyboard[0][0]
	assert.Equal(t, CabinetButtonLabel, btn.Text)
	assert.NotNil(t, btn.WebApp)
	assert.Equal(t, "https://shop.example.com/cabinet", btn.WebApp.URL)
	assert.Empty(t, btn.URL)
}

func TestCabinetKeyboard_NonHTTPS_FallsBackToURLButton(t *testing.T) {
	kb := CabinetKeyboard("http://shop.example.com/cabinet")
	assert.NotNil(t, kb)
	btn := kb.InlineKeyboard[0][0]
	assert.Equal(t, CabinetButtonLabel, btn.Text)
	assert.Nil(t, btn.WebApp)
	assert.Equal(t, "http://shop.example.com/cabinet", btn.URL)
}

func TestDisplayNameFor(t *testing.T) {
	assert.Equal(t, "First Last", displayNameFor(&models.User{FirstName: "First", LastName: "Last"}))
	assert.Equal(t, "First", displayNameFor(&models.User{FirstName: "First"}))
	assert.Equal(t, "neo", displayNameFor(&models.User{Username: "neo"}))
}
