package context

import (
	"context"

	"github.com/krateoplatformops/provider-runtime/pkg/logging"
)

type ctxKey struct{}

func CtxWithLogger(ctx context.Context, l logging.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}
func LoggerFromCtx(ctx context.Context, fallback logging.Logger) logging.Logger {
	if v := ctx.Value(ctxKey{}); v != nil {
		if l, ok := v.(logging.Logger); ok {
			return l
		}
	}
	return fallback
}
