package getters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/core-provider/internal/tools/retry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateCompositionsVersion(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(context.Background(), dyn, gvr, "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}

	updatedComp, err := dyn.Resource(gvr).Namespace(obj1.GetNamespace()).Get(context.Background(), "test-composition", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated composition: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedComp.Object, "metadata", "labels")
	if err != nil {
		t.Fatalf("failed to get labels from updated composition: %v", err)
	}

	if labels[deploy.CompositionVersionLabel] != "v2" {
		t.Errorf("expected composition version label 'v2', got '%s'", labels[deploy.CompositionVersionLabel])
	}
}

func TestUpdateCompositionsVersionRetriesTransientListError(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	listAttempts := 0
	dyn.PrependReactor("list", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		listAttempts++
		if listAttempts == 1 {
			return true, nil, fmt.Errorf("storage is (re)initializing")
		}
		return false, nil, nil
	})

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(context.Background(), dyn, gvr, "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if listAttempts != 2 {
		t.Fatalf("expected 2 list attempts, got %d", listAttempts)
	}

	updatedComp, err := dyn.Resource(gvr).Namespace(obj1.GetNamespace()).Get(context.Background(), "test-composition", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated composition: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedComp.Object, "metadata", "labels")
	if err != nil {
		t.Fatalf("failed to get labels from updated composition: %v", err)
	}

	if labels[deploy.CompositionVersionLabel] != "v2" {
		t.Errorf("expected composition version label 'v2', got '%s'", labels[deploy.CompositionVersionLabel])
	}
}

func TestUpdateCompositionsVersionRetriesTransientUpdateError(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	updateAttempts := 0
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		if updateAttempts == 1 {
			return true, nil, fmt.Errorf("storage is (re)initializing")
		}
		return false, nil, nil
	})

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(context.Background(), dyn, gvr, "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if updateAttempts != 2 {
		t.Fatalf("expected 2 update attempts, got %d", updateAttempts)
	}

	updatedComp, err := dyn.Resource(gvr).Namespace(obj1.GetNamespace()).Get(context.Background(), "test-composition", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated composition: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedComp.Object, "metadata", "labels")
	if err != nil {
		t.Fatalf("failed to get labels from updated composition: %v", err)
	}

	if labels[deploy.CompositionVersionLabel] != "v2" {
		t.Errorf("expected composition version label 'v2', got '%s'", labels[deploy.CompositionVersionLabel])
	}
}

func TestUpdateCompositionsVersionRetriesUnknownUpdateError(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	updateAttempts := 0
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		if updateAttempts == 1 {
			return true, nil, errors.New("boom")
		}
		return false, nil, nil
	})

	err := UpdateCompositionsVersion(context.Background(), dyn, testCompositionGVR(), "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if updateAttempts != 2 {
		t.Fatalf("expected 2 update attempts, got %d", updateAttempts)
	}
}

func TestUpdateCompositionsVersionSkipsUpdateWhenAlreadyOnTargetVersion(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v2")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	updateAttempts := 0
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		return false, nil, nil
	})

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(context.Background(), dyn, gvr, "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if updateAttempts != 0 {
		t.Fatalf("expected 0 update attempts, got %d", updateAttempts)
	}
}

func TestUpdateCompositionsVersionRecoversWhenFirstWriteSucceedsButReturnsTransientError(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	getAttempts := 0
	updateAttempts := 0
	dyn.PrependReactor("get", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		getAttempts++
		return false, nil, nil
	})
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		if updateAttempts != 1 {
			return false, nil, nil
		}

		u, ok := action.(clienttesting.UpdateAction).GetObject().(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("expected unstructured composition update object, got %T", action.(clienttesting.UpdateAction).GetObject())
		}
		if err := dyn.Tracker().Update(testCompositionGVR(), u.DeepCopy(), "default"); err != nil {
			t.Fatalf("failed to persist composition in tracker: %v", err)
		}

		return true, nil, fmt.Errorf("storage is (re)initializing")
	})

	err := UpdateCompositionsVersion(context.Background(), dyn, testCompositionGVR(), "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if updateAttempts != 1 {
		t.Fatalf("expected 1 update attempt, got %d", updateAttempts)
	}
	if getAttempts < 2 {
		t.Fatalf("expected at least 2 get attempts, got %d", getAttempts)
	}

	updatedComp, err := dyn.Resource(testCompositionGVR()).Namespace(obj1.GetNamespace()).Get(context.Background(), "test-composition", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated composition: %v", err)
	}

	labels, _, err := unstructured.NestedStringMap(updatedComp.Object, "metadata", "labels")
	if err != nil {
		t.Fatalf("failed to get labels from updated composition: %v", err)
	}
	if labels[deploy.CompositionVersionLabel] != "v2" {
		t.Errorf("expected composition version label 'v2', got '%s'", labels[deploy.CompositionVersionLabel])
	}
}

func TestUpdateCompositionsVersionRetriesConflictsWithFreshReads(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	getAttempts := 0
	updateAttempts := 0
	dyn.PrependReactor("get", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		getAttempts++
		return false, nil, nil
	})
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		if updateAttempts == 1 {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "composition.krateo.io", Resource: "fireworksapps"}, "test-composition", errors.New("conflict"))
		}
		return false, nil, nil
	})

	err := UpdateCompositionsVersion(context.Background(), dyn, testCompositionGVR(), "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if updateAttempts != 2 {
		t.Fatalf("expected 2 update attempts, got %d", updateAttempts)
	}
	if getAttempts < 2 {
		t.Fatalf("expected at least 2 get attempts, got %d", getAttempts)
	}
}

func TestUpdateCompositionsVersionSkipsCompositionDeletedAfterListing(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	getAttempts := 0
	updateAttempts := 0
	dyn.PrependReactor("get", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		getAttempts++
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "composition.krateo.io", Resource: "fireworksapps"}, action.(clienttesting.GetAction).GetName())
	})
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		return false, nil, nil
	})

	err := UpdateCompositionsVersion(context.Background(), dyn, testCompositionGVR(), "v2")
	if err != nil {
		t.Fatalf("updateCompositionsVersion failed: %v", err)
	}
	if getAttempts != 1 {
		t.Fatalf("expected 1 get attempt, got %d", getAttempts)
	}
	if updateAttempts != 0 {
		t.Fatalf("expected 0 update attempts, got %d", updateAttempts)
	}
}

func TestUpdateCompositionsVersionFailsFastOnPermanentUpdateError(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(context.Context, time.Duration) error { return nil }

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	updateAttempts := 0
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "composition.krateo.io", Resource: "fireworksapps"}, "test-composition", errors.New("forbidden"))
	})

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(context.Background(), dyn, gvr, "v2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if updateAttempts != 1 {
		t.Fatalf("expected 1 update attempt for permanent error, got %d", updateAttempts)
	}
}

func TestUpdateCompositionsVersionStopsRetryWhenContextCanceled(t *testing.T) {
	t.Cleanup(func() {
		retryWait = retry.Wait
	})
	retryWait = func(ctx context.Context, delay time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}

	scheme := runtime.NewScheme()
	obj1 := newTestComposition("test-composition", "default", "v0-3-0")
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "composition.krateo.io", Version: "v0-3-0", Resource: "fireworksapps"}: "TheCompositionsList",
	}, obj1)
	updateAttempts := 0
	ctx, cancel := context.WithCancel(context.Background())
	dyn.PrependReactor("update", "fireworksapps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		cancel()
		return true, nil, fmt.Errorf("storage is (re)initializing")
	})

	gvr := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}

	err := UpdateCompositionsVersion(ctx, dyn, gvr, "v2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
	if updateAttempts != 1 {
		t.Fatalf("expected 1 update attempt before cancellation, got %d", updateAttempts)
	}
}

func TestIsRetryableCompositionError(t *testing.T) {
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
		{name: "not found", err: apierrors.NewNotFound(schema.GroupResource{Group: "g", Resource: "r"}, "name"), want: false},
		{name: "conflict", err: apierrors.NewConflict(schema.GroupResource{Group: "g", Resource: "r"}, "name", errors.New("conflict")), want: true},
		{name: "unknown", err: errors.New("boom"), want: true},
		{name: "storage init", err: errors.New("storage is (re)initializing"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableCompositionError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestGetCompositionDefinitions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = compositiondefinitionsv1alpha1.SchemeBuilder.AddToScheme(scheme)

	cli := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composition-1",
				Namespace: "demo-system",
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "test-group/v1-0-0",
				Kind:       "TestKind",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "test-group",
					Kind:  "TestKind",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composition-2",
				Namespace: "default",
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "test-group/v2-0-0",
				Kind:       "OtherKind",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "test-group",
					Kind:  "OtherKind",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composition-3",
				Namespace: "krateo-system",
			},
		},
	).Build()

	gk := schema.GroupKind{
		Group: "test-group",
		Kind:  "TestKind",
	}

	compositions, err := GetCompositionDefinitions(context.Background(), cli, gk)
	if err != nil {
		t.Fatalf("getCompositionDefinitions failed: %v", err)
	}

	if len(compositions) != 1 {
		t.Fatalf("expected 1 composition, got %d", len(compositions))
	}

	if compositions[0].Name != "test-composition-1" {
		t.Errorf("expected composition name 'test-composition-1', got '%s'", compositions[0].Name)
	}
}

func TestGetCompositionDefinitionsWithVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = compositiondefinitionsv1alpha1.SchemeBuilder.AddToScheme(scheme)

	cli := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-1",
				Namespace: "ns1",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "1.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g1/v1-0-0",
				Kind:       "Kind1",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g1",
					Kind:  "Kind1",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-2",
				Namespace: "ns2",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "2.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g1/v2-0-0",
				Kind:       "Kind1",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g1",
					Kind:  "Kind1",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-3",
				Namespace: "ns3",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "1.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g2/v1-0-0",
				Kind:       "Kind2",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g2",
					Kind:  "Kind2",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-4",
				Namespace: "ns4",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "4.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g2/v4-0-0",
				Kind:       "Kind2",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g2",
					Kind:  "Kind2",
				},
			},
		},
		&compositiondefinitionsv1alpha1.CompositionDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cd-5",
				Namespace: "ns5",
			},
			Spec: compositiondefinitionsv1alpha1.CompositionDefinitionSpec{
				Chart: &compositiondefinitionsv1alpha1.ChartInfo{
					Version: "4.0.0",
				},
			},
			Status: compositiondefinitionsv1alpha1.CompositionDefinitionStatus{
				ApiVersion: "g2/v4-0-0",
				Kind:       "Kind2",
				Managed: compositiondefinitionsv1alpha1.Managed{
					Group: "g2",
					Kind:  "Kind2",
				},
			},
		},
	).Build()

	gk := schema.GroupKind{Group: "g1", Kind: "Kind1"}

	comps, err := GetCompositionDefinitionsWithVersion(context.Background(), cli, gk.WithVersion("v1-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 result, got %d", len(comps))
	}
	if comps[0].Name != "cd-1" {
		t.Errorf("expected cd-1, got %s", comps[0].Name)
	}

	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk.WithVersion("v2-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 result, got %d", len(comps))
	}
	if comps[0].Name != "cd-2" {
		t.Errorf("expected cd-2, got %s", comps[0].Name)
	}

	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk.WithVersion("v3-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected 0 results, got %d", len(comps))
	}

	gk2 := schema.GroupKind{Group: "g2", Kind: "Kind2"}
	comps, err = GetCompositionDefinitionsWithVersion(context.Background(), cli, gk2.WithVersion("v1-0-0"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 result, got %d", len(comps))
	}
	if comps[0].Name != "cd-3" {
		t.Errorf("expected cd-3, got %s", comps[0].Name)
	}
}

func newTestComposition(name, namespace, version string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "composition.krateo.io/v0-3-0",
			"kind":       "FireworksApp",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					deploy.CompositionVersionLabel: version,
				},
			},
		},
	}
}

func testCompositionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-3-0",
		Resource: "fireworksapps",
	}
}
