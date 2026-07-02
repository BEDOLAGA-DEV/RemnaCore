# botsdk — Guest SDK for RemnaCore WASM bot plugins

`botsdk` lets you write a per-shop Telegram bot as a WebAssembly plugin without
hand-rolling the host ABI. It wraps `host_call` (the `//go:wasmimport`
boilerplate, memory management, and the `{ok}`/`{error}` response envelope) and
exposes typed operations, so your plugin is pure bot logic.

## Quick start

```go
package main

import botsdk "github.com/BEDOLAGA-DEV/RemnaCore/plugins/botsdk"

func bot(u botsdk.Update) error {
	// Always register first — idempotent, and required before user-scoped ops.
	if err := botsdk.RegisterUser(u.From.ID, botsdk.DisplayName(u.From)); err != nil {
		return err
	}

	switch {
	case u.IsCallback:
		botsdk.AnswerCallback(u.CallbackID, "")
		// handle u.CallbackData …
		return nil
	case u.Text == "/plans":
		offers, err := botsdk.PlansList(botsdk.ChannelTelegram)
		if err != nil {
			return err
		}
		var kb botsdk.Keyboard
		for _, o := range offers {
			kb.Rows = append(kb.Rows, []botsdk.Button{{Text: o.Name, CallbackData: "plan:" + o.PlanID}})
		}
		return botsdk.SendKeyboard(u.ChatID, "Available plans:", kb)
	default:
		return botsdk.OpenCabinet(u.ChatID, "")
	}
}

//go:wasmexport handle_update
func handle_update() int32 { return botsdk.Handle(bot) }

func main() {}
```

See `plugins/samplebot` for a working reference plugin.

## Building

The plugin compiles to a WASI **reactor** module (so the Go runtime is
initialized before `handle_update` is called — a plain command module traps with
`runtime.notInitialized()`):

```sh
GOWORK=off GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -o bot.wasm .
```

Then install and enable it (or select it as a shop's bot via the admin/reseller
UI):

```sh
vpnctl plugin install ./bot.wasm
vpnctl plugin enable my-bot
```

## Permissions

Ops are permission-gated by the host; your plugin's `plugin.toml` must declare
the grants it uses, or the host rejects the call:

| Ops | Permission |
|-----|-----------|
| `SendText`, `SendKeyboard`, `AnswerCallback`, `EditMessage`, `OpenCabinet` | `telegram:send` |
| `RegisterUser` | `users:write` |
| `PlansList`, `PlanGet`, `SubscriptionsMine`, `SubscriptionGet`, `InvoicesPending`, `BalanceGet` | `billing:read` |
| `CheckoutCreate` | `payment:write` |
| `CancelSubscription`, `UpgradeSubscription` | `billing:write` |

Example `plugin.toml` `[telegram]` + grants section:

```toml
provides_bot = true
entry = "handle_update"
permissions = ["telegram:send", "users:write", "billing:read", "payment:write"]
```

## Ownership & tenant scoping

All user-scoped ops take a `telegramID`; the host resolves it to the shop's
customer **inside the shop's tenant scope** and enforces ownership. A bot cannot
read or mutate another user's subscription, and `UpgradeSubscription` /
`CheckoutCreate` only accept plans visible to the shop — these checks live in the
host, not the guest, so they cannot be bypassed from a plugin.

## API surface

- Telegram: `SendText`, `SendKeyboard`, `AnswerCallback`, `EditMessage`, `OpenCabinet`
- Users: `RegisterUser`, `DisplayName`
- Billing reads: `PlansList`, `PlanGet`, `SubscriptionsMine`, `SubscriptionGet`, `InvoicesPending`, `BalanceGet`
- Billing writes: `CheckoutCreate`, `CancelSubscription`, `UpgradeSubscription`
- Escape hatch: `Call(op string, args any) (json.RawMessage, error)` for ops not yet wrapped.

Op-name constants (`OpPlansList`, …) match the host `bothost.Op*` catalog.
