package context

import (
	stdctx "context"
	"testing"

	"github.com/krateoplatformops/provider-runtime/pkg/logging"
)

func TestLoggerContextHelpers(t *testing.T) {
	t.Run("CtxWithLogger stores and LoggerFromCtx retrieves the same logger", func(t *testing.T) {

		l := logging.NewNopLogger()
		ctx := CtxWithLogger(stdctx.Background(), l)
		got := LoggerFromCtx(ctx, nil)
		if got != l {
			t.Fatalf("expected logger %p, got %p", l, got)
		}
	})

	t.Run("LoggerFromCtx returns fallback when no logger present", func(t *testing.T) {
		fallback := logging.NewNopLogger()
		got := LoggerFromCtx(stdctx.Background(), fallback)
		if got != fallback {
			t.Fatalf("expected fallback %p, got %p", fallback, got)
		}
	})

	t.Run("LoggerFromCtx returns fallback when stored value has wrong type", func(t *testing.T) {
		fallback := logging.NewNopLogger()
		ctx := stdctx.WithValue(stdctx.Background(), ctxKey{}, "not-a-logger")
		got := LoggerFromCtx(ctx, fallback)
		if got != fallback {
			t.Fatalf("expected fallback %p when wrong type stored, got %p", fallback, got)
		}
	})
}
