// Package hookfn provides generic helpers for creating typed hook dispatch
// functions from the untyped [hookdispatch.Dispatcher]. These replace the
// repetitive newXxxHookFn factory boilerplate in wiring files.
//
// Each helper marshals a typed request to JSON, dispatches through the
// dispatcher, and unmarshals the response back into a typed result.
package hookfn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// NewSyncSafe creates a typed synchronous hook function that uses
// [hookdispatch.Dispatcher.DispatchSyncSafe] (compensation-aware dispatch).
// On dispatch or unmarshal failure the function returns (nil, nil), providing
// graceful degradation — hooks are optional and must never block the caller.
//
// This replaces the 6 identical newXxxHookFn factories in wiring_billing.go
// and wiring_multisub.go that follow the marshal -> DispatchSyncSafe ->
// unmarshal pattern.
func NewSyncSafe[Req any, Resp any](
	d hookdispatch.Dispatcher,
	hookName string,
	logger *slog.Logger,
) func(context.Context, Req) (*Resp, error) {
	return func(ctx context.Context, req Req) (*Resp, error) {
		data, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal %s payload: %w", hookName, err)
		}

		result := d.DispatchSyncSafe(ctx, hookName, data)
		if result.Err != nil {
			logger.Warn("hook dispatch failed, proceeding with defaults",
				slog.String("hook", hookName),
				slog.Any("error", result.Err),
			)
			return nil, nil
		}
		if result.Payload == nil {
			return nil, nil
		}

		var resp Resp
		if err := json.Unmarshal(result.Payload, &resp); err != nil {
			logger.Warn("failed to unmarshal hook response, proceeding with defaults",
				slog.String("hook", hookName),
				slog.Any("error", err),
			)
			return nil, nil
		}
		return &resp, nil
	}
}

// NewSync creates a typed synchronous hook function that uses
// [hookdispatch.Dispatcher.DispatchSync] (simple dispatch, errors propagated).
// Unlike [NewSyncSafe], dispatch and unmarshal errors are returned to the
// caller — use this for hooks where failure must abort the operation (e.g.,
// payment charge creation).
func NewSync[Req any, Resp any](
	d hookdispatch.Dispatcher,
	hookName string,
) func(context.Context, Req) (*Resp, error) {
	return func(ctx context.Context, req Req) (*Resp, error) {
		data, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal %s request: %w", hookName, err)
		}

		output, err := d.DispatchSync(ctx, hookName, data)
		if err != nil {
			return nil, err
		}

		var resp Resp
		if err := json.Unmarshal(output, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal %s result: %w", hookName, err)
		}
		return &resp, nil
	}
}

// NewSyncFireAndForget creates a typed synchronous hook function that uses
// [hookdispatch.Dispatcher.DispatchSync] but discards the response payload.
// Only the dispatch error is returned. Use this for hooks where the caller
// only cares about success/failure, not the response content (e.g., refund).
func NewSyncFireAndForget[Req any](
	d hookdispatch.Dispatcher,
	hookName string,
) func(context.Context, Req) error {
	return func(ctx context.Context, req Req) error {
		data, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal %s request: %w", hookName, err)
		}

		_, err = d.DispatchSync(ctx, hookName, data)
		return err
	}
}

// NewAsync creates a typed asynchronous hook function that marshals a
// dynamically-named payload and dispatches via
// [hookdispatch.Dispatcher.DispatchAsync] (fire-and-forget). Marshal errors
// are logged but never propagated. The hookName is provided per-call (not at
// construction time) because async hooks serve multiple dispatch points.
func NewAsync(
	d hookdispatch.Dispatcher,
	logger *slog.Logger,
) func(context.Context, string, any) {
	return func(ctx context.Context, hookName string, payload any) {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Warn("failed to marshal async hook payload",
				slog.String("hook", hookName),
				slog.Any("error", err),
			)
			return
		}
		d.DispatchAsync(ctx, hookName, data)
	}
}

// NewSyncRaw creates a synchronous hook function that marshals a
// dynamically-named payload, dispatches via
// [hookdispatch.Dispatcher.DispatchSyncSafe], and returns the raw response
// bytes without unmarshaling. Returns (nil, nil) on dispatch failure (graceful
// degradation). The hookName is provided per-call because this function serves
// multiple dispatch points.
func NewSyncRaw(
	d hookdispatch.Dispatcher,
	logger *slog.Logger,
) func(context.Context, string, any) ([]byte, error) {
	return func(ctx context.Context, hookName string, payload any) ([]byte, error) {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal hook payload: %w", err)
		}
		result := d.DispatchSyncSafe(ctx, hookName, data)
		if result.Err != nil {
			logger.Warn("hook dispatch failed, proceeding with defaults",
				slog.String("hook", hookName),
				slog.Any("error", result.Err),
			)
			return nil, nil
		}
		return result.Payload, nil
	}
}
