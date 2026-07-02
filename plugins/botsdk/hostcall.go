// Package botsdk is the guest SDK for RemnaCore WASM bot plugins. It wraps the
// host_call ABI (the //go:wasmimport boilerplate and the {ok}/{error} response
// envelope) and exposes typed operations, so a bot author writes business logic
// instead of hand-rolling memory management and JSON wire plumbing.
//
// A minimal bot:
//
//	//go:wasmexport handle_update
//	func handle_update() int32 {
//		return botsdk.Handle(func(u botsdk.Update) error {
//			if err := botsdk.RegisterUser(u.From.ID, botsdk.DisplayName(u.From)); err != nil {
//				return err
//			}
//			return botsdk.OpenCabinet(u.ChatID, "")
//		})
//	}
//
//	func main() {}
//
// Build with: GOOS=wasip1 GOARCH=wasm GOWORK=off go build -o bot.wasm .
package botsdk

import (
	"encoding/json"
	"fmt"

	pdk "github.com/extism/go-pdk"
)

// hostCall is the single host import. It accepts the Extism memory offset of a
// JSON-encoded {op,args} request and returns the offset of the JSON-encoded
// {ok}/{error} response (0 = success with no result).
//
//go:wasmimport extism:host/user host_call
func hostCall(offset uint64) uint64

// Call dispatches op with args to the host and decodes the discriminated
// {ok}/{error} response envelope. On success it returns the raw "ok" payload
// (nil when the op produced no result); on a host-side error it returns a
// non-nil error carrying the host's message.
//
// Most callers should use the typed wrappers in this package (SendText,
// PlansList, …) rather than Call directly; Call is exported for ops the typed
// surface does not yet cover.
func Call(op string, args any) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{"op": op, "args": args})
	if err != nil {
		return nil, fmt.Errorf("marshal host call %s: %w", op, err)
	}

	mem := pdk.AllocateBytes(payload)
	defer mem.Free()

	retOffset := hostCall(mem.Offset())
	if retOffset == 0 {
		return nil, nil // success, no result
	}

	retMem := pdk.FindMemory(retOffset)
	retBytes := retMem.ReadBytes()
	retMem.Free()

	var env struct {
		OK    json.RawMessage `json:"ok"`
		Error string          `json:"error"`
	}
	if uErr := json.Unmarshal(retBytes, &env); uErr != nil {
		return nil, fmt.Errorf("decode host_call %s response: %w", op, uErr)
	}
	if env.Error != "" {
		return nil, fmt.Errorf("host error from %s: %s", op, env.Error)
	}
	return env.OK, nil
}

// callInto dispatches op and unmarshals the "ok" payload into dst. A nil result
// (op with no return value) leaves dst untouched and returns nil.
func callInto(op string, args any, dst any) error {
	raw, err := Call(op, args)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode %s result: %w", op, err)
	}
	return nil
}
