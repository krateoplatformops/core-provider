package certificates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/krateoplatformops/core-provider/internal/tools/retry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPropagateCABundleWithRetryOperationRetriesRetryableErrors(t *testing.T) {
	disableCABundleRetryWait(t)

	mgr := &CertManager{
		log: func(string, ...any) {},
	}

	attempts := 0
	err := mgr.propagateCABundleWithRetryOperation(
		context.Background(),
		validTestCABundle(),
		schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "widgets"},
		func(context.Context, []byte, schema.GroupVersionResource) error {
			attempts++
			if attempts == 1 {
				return errors.New("boom")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestPropagateCABundleWithRetryOperationFailsFastOnDenylistedErrors(t *testing.T) {
	disableCABundleRetryWait(t)

	mgr := &CertManager{
		log: func(string, ...any) {},
	}

	attempts := 0
	err := mgr.propagateCABundleWithRetryOperation(
		context.Background(),
		validTestCABundle(),
		schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "widgets"},
		func(context.Context, []byte, schema.GroupVersionResource) error {
			attempts++
			return apierrors.NewForbidden(schema.GroupResource{Group: "g", Resource: "widgets"}, "widgets", errors.New("forbidden"))
		},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestPropagateCABundleWithRetryOperationRejectsInvalidCABundleBeforeRetry(t *testing.T) {
	disableCABundleRetryWait(t)

	mgr := &CertManager{
		log: func(string, ...any) {},
	}

	attempts := 0
	err := mgr.propagateCABundleWithRetryOperation(
		context.Background(),
		[]byte("not pem"),
		schema.GroupVersionResource{Group: "g", Version: "v1", Resource: "widgets"},
		func(context.Context, []byte, schema.GroupVersionResource) error {
			attempts++
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 0 {
		t.Fatalf("expected 0 attempts, got %d", attempts)
	}
}

func TestIsRetryableCABundleError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Group: "g", Resource: "widgets"}, "widgets", errors.New("forbidden")), want: false},
		{name: "bad request", err: apierrors.NewBadRequest("bad request"), want: false},
		{name: "not found", err: apierrors.NewNotFound(schema.GroupResource{Group: "g", Resource: "widgets"}, "widgets"), want: true},
		{name: "conflict", err: apierrors.NewConflict(schema.GroupResource{Group: "g", Resource: "widgets"}, "widgets", errors.New("conflict")), want: true},
		{name: "unknown", err: errors.New("boom"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableCABundleError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func disableCABundleRetryWait(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		caBundleRetryWait = retry.Wait
	})
	caBundleRetryWait = func(context.Context, time.Duration) error { return nil }
}

func validTestCABundle() []byte {
	return []byte("-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n")
}
