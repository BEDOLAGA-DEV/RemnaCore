package telegram

import (
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToInlineKeyboard_WebAppButton(t *testing.T) {
	kb := bothost.Keyboard{
		Rows: [][]bothost.Button{
			{{Text: "Cabinet", WebAppURL: "https://x.example.com"}},
		},
	}
	got := toInlineKeyboard(kb)
	require.NotNil(t, got)
	require.Len(t, got.InlineKeyboard, 1)
	require.Len(t, got.InlineKeyboard[0], 1)
	btn := got.InlineKeyboard[0][0]
	assert.Equal(t, "Cabinet", btn.Text)
	require.NotNil(t, btn.WebApp)
	assert.Equal(t, "https://x.example.com", btn.WebApp.URL)
	assert.Empty(t, btn.URL)
	assert.Empty(t, btn.CallbackData)
}

func TestToInlineKeyboard_URLButton(t *testing.T) {
	kb := bothost.Keyboard{
		Rows: [][]bothost.Button{
			{{Text: "Visit", URL: "https://example.com"}},
		},
	}
	got := toInlineKeyboard(kb)
	require.NotNil(t, got)
	require.Len(t, got.InlineKeyboard, 1)
	btn := got.InlineKeyboard[0][0]
	assert.Equal(t, "Visit", btn.Text)
	assert.Equal(t, "https://example.com", btn.URL)
	assert.Nil(t, btn.WebApp)
	assert.Empty(t, btn.CallbackData)
}

func TestToInlineKeyboard_CallbackButton(t *testing.T) {
	kb := bothost.Keyboard{
		Rows: [][]bothost.Button{
			{{Text: "Click", CallbackData: "action:1"}},
		},
	}
	got := toInlineKeyboard(kb)
	require.NotNil(t, got)
	require.Len(t, got.InlineKeyboard, 1)
	btn := got.InlineKeyboard[0][0]
	assert.Equal(t, "Click", btn.Text)
	assert.Equal(t, "action:1", btn.CallbackData)
	assert.Nil(t, btn.WebApp)
	assert.Empty(t, btn.URL)
}

func TestToInlineKeyboard_PreservesRowColumnShape(t *testing.T) {
	kb := bothost.Keyboard{
		Rows: [][]bothost.Button{
			{
				{Text: "R0C0", CallbackData: "r0c0"},
				{Text: "R0C1", CallbackData: "r0c1"},
			},
			{
				{Text: "R1C0", URL: "https://a.example.com"},
				{Text: "R1C1", URL: "https://b.example.com"},
			},
		},
	}
	got := toInlineKeyboard(kb)
	require.NotNil(t, got)
	assert.Len(t, got.InlineKeyboard, 2)
	assert.Len(t, got.InlineKeyboard[0], 2)
	assert.Len(t, got.InlineKeyboard[1], 2)
	assert.Equal(t, "R0C0", got.InlineKeyboard[0][0].Text)
	assert.Equal(t, "r0c0", got.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "R1C1", got.InlineKeyboard[1][1].Text)
	assert.Equal(t, "https://b.example.com", got.InlineKeyboard[1][1].URL)
}
