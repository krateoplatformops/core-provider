package compositiondefinitions

import (
	"context"
	"errors"
	"testing"
	"time"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/retry"
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateCompositionDefinitionStatusWithRetryRetriesConflicts(t *testing.T) {
	disableCompositionDefinitionRetryWait(t)

	fake, raw := newRetryTestClient(t, newTestCompositionDefinition())
	statusAttempts := 0
	fake.onStatusUpdate = func(ctx context.Context, obj client.Object, next func() error, _ ...client.SubResourceUpdateOption) error {
		statusAttempts++
		if statusAttempts == 1 {
			return apierrors.NewConflict(schema.GroupResource{Group: compositiondefinitionsv1alpha1.Group, Resource: "compositiondefinitions"}, obj.GetName(), errors.New("conflict"))
		}
		return next()
	}

	ext := &external{kube: fake}
	err := ext.updateCompositionDefinitionStatusWithRetry(context.Background(), "default", "test", func(current *compositiondefinitionsv1alpha1.CompositionDefinition) {
		current.Status.Digest = "digest-1"
		current.SetConditions(rtv1.Available())
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if statusAttempts != 2 {
		t.Fatalf("expected 2 status update attempts, got %d", statusAttempts)
	}
	if fake.getCalls < 2 {
		t.Fatalf("expected at least 2 get calls, got %d", fake.getCalls)
	}

	stored := &compositiondefinitionsv1alpha1.CompositionDefinition{}
	if err := raw.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test"}, stored); err != nil {
		t.Fatalf("failed to get CompositionDefinition: %v", err)
	}
	if stored.Status.Digest != "digest-1" {
		t.Fatalf("expected digest-1, got %q", stored.Status.Digest)
	}
}

func TestUpdateCompositionDefinitionStatusWithRetryAvoidsDuplicateWriteAfterAmbiguousSuccess(t *testing.T) {
	disableCompositionDefinitionRetryWait(t)

	fake, raw := newRetryTestClient(t, newTestCompositionDefinition())
	statusAttempts := 0
	fake.onStatusUpdate = func(ctx context.Context, obj client.Object, next func() error, _ ...client.SubResourceUpdateOption) error {
		statusAttempts++
		if statusAttempts == 1 {
			if err := next(); err != nil {
				return err
			}
			return errors.New("boom")
		}
		return next()
	}

	ext := &external{kube: fake}
	err := ext.updateCompositionDefinitionStatusWithRetry(context.Background(), "default", "test", func(current *compositiondefinitionsv1alpha1.CompositionDefinition) {
		current.Status.Digest = "digest-1"
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if statusAttempts != 1 {
		t.Fatalf("expected 1 status update attempt, got %d", statusAttempts)
	}
	if fake.getCalls < 2 {
		t.Fatalf("expected at least 2 get calls, got %d", fake.getCalls)
	}

	stored := &compositiondefinitionsv1alpha1.CompositionDefinition{}
	if err := raw.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test"}, stored); err != nil {
		t.Fatalf("failed to get CompositionDefinition: %v", err)
	}
	if stored.Status.Digest != "digest-1" {
		t.Fatalf("expected digest-1, got %q", stored.Status.Digest)
	}
}

func TestUpdateCompositionDefinitionWithRetryRetriesConflicts(t *testing.T) {
	disableCompositionDefinitionRetryWait(t)

	fake, raw := newRetryTestClient(t, newTestCompositionDefinition())
	updateAttempts := 0
	fake.onUpdate = func(ctx context.Context, obj client.Object, next func() error, _ ...client.UpdateOption) error {
		updateAttempts++
		if updateAttempts == 1 {
			return apierrors.NewConflict(schema.GroupResource{Group: compositiondefinitionsv1alpha1.Group, Resource: "compositiondefinitions"}, obj.GetName(), errors.New("conflict"))
		}
		return next()
	}

	ext := &external{kube: fake}
	err := ext.updateCompositionDefinitionWithRetry(context.Background(), "default", "test", func(current *compositiondefinitionsv1alpha1.CompositionDefinition) bool {
		if meta.FinalizerExists(current, compositionStillExistFinalizer) {
			return false
		}
		meta.AddFinalizer(current, compositionStillExistFinalizer)
		return true
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if updateAttempts != 2 {
		t.Fatalf("expected 2 update attempts, got %d", updateAttempts)
	}
	if fake.getCalls < 2 {
		t.Fatalf("expected at least 2 get calls, got %d", fake.getCalls)
	}

	stored := &compositiondefinitionsv1alpha1.CompositionDefinition{}
	if err := raw.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test"}, stored); err != nil {
		t.Fatalf("failed to get CompositionDefinition: %v", err)
	}
	if !meta.FinalizerExists(stored, compositionStillExistFinalizer) {
		t.Fatal("expected finalizer to be present")
	}
}

func TestUpdateCompositionDefinitionWithRetryAvoidsDuplicateWriteAfterAmbiguousSuccess(t *testing.T) {
	disableCompositionDefinitionRetryWait(t)

	fake, raw := newRetryTestClient(t, newTestCompositionDefinition())
	updateAttempts := 0
	fake.onUpdate = func(ctx context.Context, obj client.Object, next func() error, _ ...client.UpdateOption) error {
		updateAttempts++
		if updateAttempts == 1 {
			if err := next(); err != nil {
				return err
			}
			return errors.New("boom")
		}
		return next()
	}

	ext := &external{kube: fake}
	err := ext.updateCompositionDefinitionWithRetry(context.Background(), "default", "test", func(current *compositiondefinitionsv1alpha1.CompositionDefinition) bool {
		if meta.FinalizerExists(current, compositionStillExistFinalizer) {
			return false
		}
		meta.AddFinalizer(current, compositionStillExistFinalizer)
		return true
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if updateAttempts != 1 {
		t.Fatalf("expected 1 update attempt, got %d", updateAttempts)
	}
	if fake.getCalls < 2 {
		t.Fatalf("expected at least 2 get calls, got %d", fake.getCalls)
	}

	stored := &compositiondefinitionsv1alpha1.CompositionDefinition{}
	if err := raw.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test"}, stored); err != nil {
		t.Fatalf("failed to get CompositionDefinition: %v", err)
	}
	if !meta.FinalizerExists(stored, compositionStillExistFinalizer) {
		t.Fatal("expected finalizer to be present")
	}
}

func newRetryTestClient(t *testing.T, obj *compositiondefinitionsv1alpha1.CompositionDefinition) (*retryTestClient, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := compositiondefinitionsv1alpha1.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	raw := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&compositiondefinitionsv1alpha1.CompositionDefinition{}).
		WithObjects(obj).
		Build()

	return &retryTestClient{Client: raw}, raw
}

func newTestCompositionDefinition() *compositiondefinitionsv1alpha1.CompositionDefinition {
	return &compositiondefinitionsv1alpha1.CompositionDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}
}

type retryTestClient struct {
	client.Client

	getCalls          int
	updateCalls       int
	statusUpdateCalls int

	onGet          func(context.Context, client.ObjectKey, client.Object, func() error, ...client.GetOption) error
	onUpdate       func(context.Context, client.Object, func() error, ...client.UpdateOption) error
	onStatusUpdate func(context.Context, client.Object, func() error, ...client.SubResourceUpdateOption) error
}

func (c *retryTestClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalls++
	next := func() error {
		return c.Client.Get(ctx, key, obj, opts...)
	}
	if c.onGet != nil {
		return c.onGet(ctx, key, obj, next, opts...)
	}
	return next()
}

func (c *retryTestClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCalls++
	next := func() error {
		return c.Client.Update(ctx, obj, opts...)
	}
	if c.onUpdate != nil {
		return c.onUpdate(ctx, obj, next, opts...)
	}
	return next()
}

func (c *retryTestClient) Status() client.StatusWriter {
	return &retryTestStatusWriter{
		SubResourceWriter: c.Client.Status(),
		parent:            c,
	}
}

type retryTestStatusWriter struct {
	client.SubResourceWriter
	parent *retryTestClient
}

func (w *retryTestStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	w.parent.statusUpdateCalls++
	next := func() error {
		return w.SubResourceWriter.Update(ctx, obj, opts...)
	}
	if w.parent.onStatusUpdate != nil {
		return w.parent.onStatusUpdate(ctx, obj, next, opts...)
	}
	return next()
}

func TestIsRetryableCompositionDefinitionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Group: "g", Resource: "r"}, "name", errors.New("forbidden")), want: false},
		{name: "bad request", err: apierrors.NewBadRequest("bad request"), want: false},
		{name: "conflict", err: apierrors.NewConflict(schema.GroupResource{Group: "g", Resource: "r"}, "name", errors.New("conflict")), want: true},
		{name: "unknown", err: errors.New("boom"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableCompositionDefinitionError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func disableCompositionDefinitionRetryWait(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		compositionDefinitionRetryWait = retry.Wait
	})
	compositionDefinitionRetryWait = func(context.Context, time.Duration) error { return nil }
}
