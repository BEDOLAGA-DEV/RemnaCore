package botsdk

import (
	"encoding/json"
	"fmt"
	"strings"

	pdk "github.com/extism/go-pdk"
)

// Handle is the entry-point adapter for a bot's exported handle_update function.
// It reads the inbound Update from the host, invokes fn, and translates the
// result into the host's status convention (0 = ok, 1 = error with the message
// surfaced via pdk.SetError). On success it writes "ok" as the output.
//
// Usage:
//
//	//go:wasmexport handle_update
//	func handle_update() int32 { return botsdk.Handle(myBot) }
func Handle(fn func(Update) error) int32 {
	var upd Update
	if err := json.Unmarshal(pdk.Input(), &upd); err != nil {
		pdk.SetError(fmt.Errorf("botsdk: unmarshal update: %w", err))
		return 1
	}
	if err := fn(upd); err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.OutputString("ok")
	return 0
}

// Command extracts the leading slash-command token from an update's text,
// stripping an @botname suffix ("/plans@shop_bot arg" → "/plans"). Returns ""
// for non-command text.
func Command(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	cmd, _, _ := strings.Cut(fields[0], "@")
	return cmd
}
