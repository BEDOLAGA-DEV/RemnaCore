package cabinetbot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
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
	// results maps op → canned JSON result returned by Call (nil map → nil result).
	results map[string]json.RawMessage
}

func (h *fakeHost) Call(_ context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	h.calls = append(h.calls, recordedCall{op: op, args: args})
	if op == h.failOp {
		return nil, h.failErr
	}
	return h.results[op], nil
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

// ─── Router tests ────────────────────────────────────────────────────────────

func TestCommand_Parsing(t *testing.T) {
	require.Equal(t, "/plans", command("/plans"))
	require.Equal(t, "/plans", command("/plans@shop_bot arg"))
	require.Equal(t, "/my", command("  /my  "))
	require.Equal(t, "", command("hello"))
	require.Equal(t, "", command(""))
}

func TestHandler_Plans_SendsOfferKeyboard(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpPlansList: json.RawMessage(`[{"plan_id":"p1","name":"Basic","periods":[{"days":30,"amount":999,"currency":"USD","label":"1 месяц","plan_id":"p1"}]}]`),
	}}
	update := bothost.Update{ChatID: 5, From: bothost.User{ID: 9, Username: "neo"}, Text: "/plans"}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpPlansList, bothost.OpTelegramSendKeyboard}, host.ops())

	var kbArgs struct {
		ChatID   int64            `json:"chat_id"`
		Keyboard bothost.Keyboard `json:"keyboard"`
	}
	require.NoError(t, json.Unmarshal(host.calls[2].args, &kbArgs))
	require.EqualValues(t, 5, kbArgs.ChatID)
	require.Len(t, kbArgs.Keyboard.Rows, 1)
	require.Equal(t, "plan:p1", kbArgs.Keyboard.Rows[0][0].CallbackData)
}

func TestHandler_Plans_EmptyList_SendsText(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpPlansList: json.RawMessage(`[]`),
	}}
	update := bothost.Update{ChatID: 5, From: bothost.User{ID: 9}, Text: "/plans"}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpPlansList, bothost.OpTelegramSendText}, host.ops())
}

func TestHandler_My_ResolvesPlanNames(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpSubscriptionsMine: json.RawMessage(`[{"id":"s1","plan_id":"p1","status":"active"}]`),
		bothost.OpPlansGet:          json.RawMessage(`{"plan_id":"p1","name":"Basic"}`),
	}}
	update := bothost.Update{ChatID: 5, From: bothost.User{ID: 9}, Text: "/my"}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpSubscriptionsMine, bothost.OpPlansGet, bothost.OpTelegramSendText}, host.ops())

	var sendArgs struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(host.calls[3].args, &sendArgs))
	require.Contains(t, sendArgs.Text, "Basic — active")
}

func TestHandler_Balance_SendsWallets(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpBalanceGet: json.RawMessage(`[{"kind":"main","currency":"USD","balance":1500,"available":1000}]`),
	}}
	update := bothost.Update{ChatID: 5, From: bothost.User{ID: 9}, Text: "/balance"}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpBalanceGet, bothost.OpTelegramSendText}, host.ops())
}

func TestHandler_Invoices_SendsPending(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpInvoicesPending: json.RawMessage(`[{"id":"inv-1","status":"pending","amount":999,"currency":"USD"}]`),
	}}
	update := bothost.Update{ChatID: 5, From: bothost.User{ID: 9}, Text: "/invoices"}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpInvoicesPending, bothost.OpTelegramSendText}, host.ops())
}

func TestHandler_CommandOpFails_NotifiesAndReturnsError(t *testing.T) {
	opErr := errors.New("reader down")
	host := &fakeHost{failOp: bothost.OpSubscriptionsMine, failErr: opErr}
	update := bothost.Update{ChatID: 5, From: bothost.User{ID: 9}, Text: "/my"}

	err := Handler(context.Background(), update, host)
	require.ErrorIs(t, err, opErr)
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpSubscriptionsMine, bothost.OpTelegramSendText}, host.ops())

	var sendArgs struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(host.calls[2].args, &sendArgs))
	require.Equal(t, msgCommandFailed, sendArgs.Text)
}

func TestHandler_PlanCallback_SendsDetailWithBuyButtons(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpPlansGet: json.RawMessage(`{"plan_id":"p1","name":"Basic","periods":[{"days":30,"amount":999,"currency":"USD","label":"1 месяц","plan_id":"period-30"}]}`),
	}}
	update := bothost.Update{
		ChatID:       5,
		From:         bothost.User{ID: 9},
		IsCallback:   true,
		CallbackID:   "cb-1",
		CallbackData: "plan:p1",
	}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpTelegramAnswerCallback, bothost.OpPlansGet, bothost.OpTelegramSendKeyboard}, host.ops())

	var kbArgs struct {
		Keyboard bothost.Keyboard `json:"keyboard"`
	}
	require.NoError(t, json.Unmarshal(host.calls[3].args, &kbArgs))
	require.Equal(t, "buy:period-30", kbArgs.Keyboard.Rows[0][0].CallbackData)
}

func TestHandler_BuyCallback_StartsCheckoutAndSendsPayButton(t *testing.T) {
	host := &fakeHost{results: map[string]json.RawMessage{
		bothost.OpCheckoutCreate: json.RawMessage(`{"checkout_url":"https://pay.example.com/x","subscription_id":"s1","invoice_id":"i1","provider":"stripe"}`),
	}}
	update := bothost.Update{
		ChatID:       5,
		From:         bothost.User{ID: 9},
		IsCallback:   true,
		CallbackID:   "cb-2",
		CallbackData: "buy:period-30",
	}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpTelegramAnswerCallback, bothost.OpCheckoutCreate, bothost.OpTelegramSendKeyboard}, host.ops())

	var coArgs struct {
		TelegramID int64  `json:"telegram_id"`
		PlanID     string `json:"plan_id"`
	}
	require.NoError(t, json.Unmarshal(host.calls[2].args, &coArgs))
	require.EqualValues(t, 9, coArgs.TelegramID)
	require.Equal(t, "period-30", coArgs.PlanID)

	var kbArgs struct {
		Keyboard bothost.Keyboard `json:"keyboard"`
	}
	require.NoError(t, json.Unmarshal(host.calls[3].args, &kbArgs))
	require.Equal(t, "https://pay.example.com/x", kbArgs.Keyboard.Rows[0][0].URL)
}

func TestHandler_BuyCallback_CheckoutFails_Notifies(t *testing.T) {
	coErr := errors.New("no provider")
	host := &fakeHost{failOp: bothost.OpCheckoutCreate, failErr: coErr}
	update := bothost.Update{
		ChatID:       5,
		From:         bothost.User{ID: 9},
		IsCallback:   true,
		CallbackID:   "cb-3",
		CallbackData: "buy:p1",
	}

	err := Handler(context.Background(), update, host)
	require.ErrorIs(t, err, coErr)
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpTelegramAnswerCallback, bothost.OpCheckoutCreate, bothost.OpTelegramSendText}, host.ops())
}

func TestHandler_UnknownCallback_AnswersStale(t *testing.T) {
	host := &fakeHost{}
	update := bothost.Update{
		ChatID:       5,
		From:         bothost.User{ID: 9},
		IsCallback:   true,
		CallbackID:   "cb-4",
		CallbackData: "addon:whatever",
	}

	require.NoError(t, Handler(context.Background(), update, host))
	require.Equal(t, []string{bothost.OpUserRegister, bothost.OpTelegramAnswerCallback}, host.ops())

	var ansArgs struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(host.calls[1].args, &ansArgs))
	require.Equal(t, msgStaleKeyboard, ansArgs.Text)
}

func TestRequiredPerms_IncludeBillingAndPayment(t *testing.T) {
	perms := RequiredPerms()
	require.True(t, perms.Has(plugin.PermTelegramSend))
	require.True(t, perms.Has(plugin.PermUsersWrite))
	require.True(t, perms.Has(plugin.PermBillingRead))
	require.True(t, perms.Has(plugin.PermPaymentWrite))
}
