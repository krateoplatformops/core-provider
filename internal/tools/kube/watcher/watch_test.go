//go:build integration
// +build integration

package watcher

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"

	// if you have e2e helpers; otherwise remove
	xenv "github.com/krateoplatformops/plumbing/env"
)

var (
	testenv     env.Environment
	clusterName string
	namespace   string
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	namespace = "demo-system"
	clusterName = "krateo-watcher"
	testenv = env.New()

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), clusterName),
		envfuncs.CreateNamespace(namespace),
	).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}

// ---- Tests ----

func TestWatcher_ImmediateReady_E2E(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Watcher Immediate Ready - e2e").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		}).Assess("ImmediateReady", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		ns := namespace
		name := "ready-deploy-e2e"

		dyn, err := dynamic.NewForConfig(cfg.Client().RESTConfig())
		if err != nil {
			t.Fatalf("failed to create dynamic client: %v", err)
		}

		readyObj := makeDeploymentUnstructuredObj(name, ns, true)
		_, err = dyn.Resource(gvr).Namespace(ns).Create(ctx, readyObj, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create deployment: %v", err)
		}

		time.Sleep(20 * time.Second)

		w := NewWatcher(dyn, gvr, 5*time.Second, isAvailableChecker)

		if err := w.WatchResource(ctx, ns, name); err != nil {
			t.Fatalf("expected ready, got error: %v", err)
		}

		// cleanup
		_ = dyn.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})

		return ctx
	}).Feature()

	testenv.Test(t, f)
}

func TestWatcher_BecomesReadyAfterModify_E2E(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Watcher Becomes Ready After Modify - e2e").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		}).Assess("BecomesReadyAfterModify", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		ns := namespace
		name := "eventual-deploy-e2e"

		dyn, err := dynamic.NewForConfig(cfg.Client().RESTConfig())
		if err != nil {
			t.Fatalf("failed to create dynamic client: %v", err)
		}

		initial := makeDeploymentUnstructuredObj(name, ns, false)
		_, err = dyn.Resource(gvr).Namespace(ns).Create(ctx, initial, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create initial deploy: %v", err)
		}

		w := NewWatcher(dyn, gvr, 10*time.Second, isAvailableChecker)

		// after a short delay, update the object to set Available=True
		go func() {
			time.Sleep(500 * time.Millisecond)
			updated := makeDeploymentUnstructuredObj(name, ns, true)
			// try to preserve resourceVersion by fetching current, then set fields
			cur, err := dyn.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
			if err == nil {
				updated.SetResourceVersion(cur.GetResourceVersion())
			}
			_, _ = dyn.Resource(gvr).Namespace(ns).Update(context.Background(), updated, metav1.UpdateOptions{})
		}()

		if err := w.WatchResource(ctx, ns, name); err != nil {
			t.Fatalf("expected eventual ready, got error: %v", err)
		}

		// cleanup
		_ = dyn.Resource(gvr).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})

		return ctx
	}).Feature()

	testenv.Test(t, f)
}

func TestWatcher_Timeout_E2E(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Watcher Timeout - e2e").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		}).Assess("Timeout", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		ns := namespace
		name := "never-ready-e2e"

		dyn, err := dynamic.NewForConfig(cfg.Client().RESTConfig())
		if err != nil {
			t.Fatalf("failed to create dynamic client: %v", err)
		}

		w := NewWatcher(dyn, gvr, 500*time.Millisecond, isAvailableChecker)

		if err := w.WatchResource(ctx, ns, name); err == nil {
			t.Fatalf("expected timeout error, got nil")
		}

		return ctx
	}).Feature()

	testenv.Test(t, f)
}

func TestWatcher_DeletedDuringWatch_E2E(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Watcher Deleted During Watch - e2e").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		}).Assess("DeletedDuringWatch", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
		ns := namespace
		name := "will-delete-e2e"

		dyn, err := dynamic.NewForConfig(cfg.Client().RESTConfig())
		if err != nil {
			t.Fatalf("failed to create dynamic client: %v", err)
		}

		initial := makeDeploymentUnstructuredObj(name, ns, false)
		_, err = dyn.Resource(gvr).Namespace(ns).Create(ctx, initial, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create initial deploy: %v", err)
		}

		w := NewWatcher(dyn, gvr, 5*time.Second, isAvailableChecker)

		// delete the object after a short delay to simulate a delete event
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = dyn.Resource(gvr).Namespace(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
		}()

		if err := w.WatchResource(ctx, ns, name); err == nil {
			t.Fatalf("expected deletion error, got nil")
		}

		return ctx
	}).Feature()

	testenv.Test(t, f)
}

func makeDeploymentUnstructuredObj(name, namespace string, available bool) *unstructured.Unstructured {
	conds := []interface{}{}
	if available {
		conds = append(conds, map[string]interface{}{
			"type":   "Available",
			"status": "True",
		})
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": name,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": name,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:latest",
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"conditions": conds,
			},
		},
	}
}

func isAvailableChecker(u *appsv1.Deployment) bool {
	mu, err := runtime.DefaultUnstructuredConverter.ToUnstructured(u.DeepCopyObject())
	if err != nil {
		return false
	}
	uns := &unstructured.Unstructured{Object: mu}
	conds, found, _ := unstructured.NestedSlice(uns.Object, "status", "conditions")
	if !found {
		fmt.Println("conditions not found")
		return false
	}
	for _, c := range conds {
		fmt.Println("condition:", c)
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == "Available" {
			if s, _ := m["status"].(string); s == "True" {
				return true
			}
		}
	}
	return false
}
