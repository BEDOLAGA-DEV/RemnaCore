package plugin

import "context"

// flowBindingsKey is the context key for storing FlowBindings.
type flowBindingsKey struct{}

// FlowBindings maps plugin slugs to their pinned pool versions. When attached
// to a context via WithFlowBindings, subsequent CallHook invocations will
// prefer the pinned pool version for each slug, ensuring version consistency
// across a multi-hook business flow.
type FlowBindings map[string]uint64

// WithFlowBindings attaches version bindings to the context. Subsequent
// CallHook calls will prefer the pinned pool version for each slug.
func WithFlowBindings(ctx context.Context, bindings FlowBindings) context.Context {
	return context.WithValue(ctx, flowBindingsKey{}, bindings)
}

// flowBindingsFromContext extracts flow bindings from the context, or nil if
// not set.
func flowBindingsFromContext(ctx context.Context) FlowBindings {
	v, _ := ctx.Value(flowBindingsKey{}).(FlowBindings)
	return v
}
