package cabinetbot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

type recordedCall struct {
	op   string
	args json.RawMessage
}

type fakeHost struct {
	calls   []recordedCall
	failOp  string
	failErr error
}

func (h *fakeHost) Call(_ context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	h.calls = append(h.calls, recordedCall{op: op, args: args})
	if op == h.failOp {
		return nil, h.failErr
	}
	return nil, nil
}

func (h *fakeHost) ops() []string {
	out := make([]string, len(h.calls))
	for i, c := range h.calls {
		out[i] = c.op
	}
	return out
}

func TestHandler_HappyPath_RegistersThenOpensCabinet(t *testing.T) {
	host := &fakeHost{}
	update := bothost.Update{
		ChatID: 4242,
		From:   bothost.User{ID: 99, FirstName: "Neo", LastName: "Anderson"},
		Text:   "/start",
	}

	require.NoError(t, Handler(context.Background(), update, host))

	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpCabinetOpen}, host.ops())

	var reg map[string]any
	require.NoError(t, json.Unmarshal(host.calls[0].args, &reg))
	require.EqualValues(t, 99, reg["telegram_id"])
	require.Equal(t, "Neo Anderson", reg["display_name"])

	var open map[string]any
	require.NoError(t, json.Unmarshal(host.calls[1].args, &open))
	require.EqualValues(t, 4242, open["chat_id"])
}

func TestHandler_RegisterFails_NotifiesAndStops(t *testing.T) {
	regErr := errors.New("boom")
	host := &fakeHost{failOp: bothost.OpUserRegister, failErr: regErr}
	update := bothost.Update{ChatID: 7, From: bothost.User{ID: 1, Username: "neo"}}

	err := Handler(context.Background(), update, host)
	require.ErrorIs(t, err, regErr)

	// register attempted, then a failure notice sent; cabinet NOT opened.
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpTelegramSendText}, host.ops())
	require.NotContains(t, host.ops(), bothost.OpCabinetOpen)

	var notice map[string]any
	require.NoError(t, json.Unmarshal(host.calls[1].args, &notice))
	require.EqualValues(t, 7, notice["chat_id"])
	require.Equal(t, msgRegistrationFailed, notice["text"])
}

func TestPlugin_DeclaresBotCapability(t *testing.T) {
	def := Plugin()
	require.Equal(t, Slug, def.Slug)
	require.True(t, def.ProvidesBot)
}
