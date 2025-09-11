package app

import "context"

type noFanoutKey struct{}

func WithNoFanout(ctx context.Context) context.Context {
    return context.WithValue(ctx, noFanoutKey{}, true)
}
func NoFanout(ctx context.Context) bool {
    v, _ := ctx.Value(noFanoutKey{}).(bool)
    return v
}