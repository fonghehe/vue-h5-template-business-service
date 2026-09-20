// Package requestctx carries request metadata across HTTP, business and SQL
// code without importing Gin or the response writer.
package requestctx

import "context"

type key struct{}

func WithID(ctx context.Context, id string) context.Context { return context.WithValue(ctx, key{}, id) }
func ID(ctx context.Context) string                         { value, _ := ctx.Value(key{}).(string); return value }
