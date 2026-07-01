// Package main implements the sample WASM bot plugin for RemnaCore BP2.
//
// This guest module exports handle_update and imports host_call from the
// extism:host/user module. It is compiled with:
//
//	GOOS=wasip1 GOARCH=wasm go build -o samplebot.wasm .
package main

import (
	"encoding/json"
	"fmt"

	pdk "github.com/extism/go-pdk"
)

// hostCall is the single host import. It accepts the Extism memory offset of
// a JSON-encoded request and returns the offset of the JSON-encoded response.
//
//go:wasmimport extism:host/user host_call
func hostCall(offset uint64) uint64

// Op name constants — no string literals in business logic.
const (
	opUserRegister = "user.register"
	opCabinetOpen  = "cabinet.open"
)

// outputOK is the fixed output written on success.
const outputOK = "ok"

// Update mirrors bothost.Update for JSON decoding inside the WASM guest.
type Update struct {
	ChatID int64  `json:"chat_id"`
	From   Sender `json:"from"`
	Text   string `json:"text"`
}

// Sender is the nested "from" field of an Update.
type Sender struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// callHost marshals {"op": op, "args": args}, sends it to the host via
// host_call, and decodes the discriminated {ok}/{error} envelope the host
// returns. retOffset==0 means success with no result; a non-zero offset
// carries either {"ok": <result>} or {"error": "<msg>"}.
func callHost(op string, args any) (json.RawMessage, error) {
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

// buildDisplayName derives the user's display name: FirstName, plus LastName
// when present, falling back to Username only when the name is otherwise empty.
// Mirrors bothost.DisplayName so the WASM and built-in bots behave identically.
func buildDisplayName(s Sender) string {
	name := s.FirstName
	if s.LastName != "" {
		name += " " + s.LastName
	}
	if name == "" {
		name = s.Username
	}
	return name
}

// handle_update is the exported entry point called by the bot host dispatcher.
// It reads a bothost.Update JSON from the Extism input, registers the user,
// opens the cabinet, and sets the output to "ok".
//
//go:wasmexport handle_update
func handle_update() int32 {
	var upd Update
	if err := json.Unmarshal(pdk.Input(), &upd); err != nil {
		pdk.SetError(fmt.Errorf("unmarshal update: %w", err))
		return 1
	}

	name := buildDisplayName(upd.From)

	if _, err := callHost(opUserRegister, map[string]any{
		"telegram_id":  upd.From.ID,
		"display_name": name,
	}); err != nil {
		pdk.SetError(err)
		return 1
	}

	if _, err := callHost(opCabinetOpen, map[string]any{
		"chat_id": upd.ChatID,
	}); err != nil {
		pdk.SetError(err)
		return 1
	}

	pdk.OutputString(outputOK)
	return 0
}

func main() {}
