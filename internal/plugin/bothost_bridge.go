package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// BotHostBridge dispatches one bot op. Its signature is identical to
// bothost.Host.Call so a bothost.Host satisfies it structurally; defined here
// (NOT importing bothost) to avoid the plugin<-bothost import cycle.
type BotHostBridge interface {
	Call(ctx context.Context, op string, args json.RawMessage) (json.RawMessage, error)
}

type botHostCtxKey struct{}

// WithBotHost returns a ctx carrying h for host_call to use during one dispatch.
func WithBotHost(ctx context.Context, h BotHostBridge) context.Context {
	return context.WithValue(ctx, botHostCtxKey{}, h)
}

func botHostFromContext(ctx context.Context) (BotHostBridge, bool) {
	h, ok := ctx.Value(botHostCtxKey{}).(BotHostBridge)
	return h, ok
}

// ErrNoBotHost is returned by handleHostCall when no BotHostBridge is in ctx
// (e.g. a non-bot plugin called host_call).
var ErrNoBotHost = errors.New("host_call: no bot host in context")

// handleHostCall parses the guest's {op,args} request, dispatches it to the
// BotHostBridge in ctx, and returns the op's JSON result (or an error).
func handleHostCall(ctx context.Context, input []byte) ([]byte, error) {
	var req struct {
		Op   string          `json:"op"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("host_call: decode request: %w", err)
	}
	bridge, ok := botHostFromContext(ctx)
	if !ok {
		return nil, ErrNoBotHost
	}
	res, err := bridge.Call(ctx, req.Op, req.Args)
	if err != nil {
		return nil, err
	}
	return res, nil
}
