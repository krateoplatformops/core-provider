//go:build integration
// +build integration

package compositiondefinitions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/krateoplatformops/core-provider/apis"
	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/plumbing/e2e"
	xenv "github.com/krateoplatformops/plumbing/env"
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/utils"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	testenv     env.Environment
	clusterName string
	namespace   string
)

const (
	crdPath       = "../../../crds"
	testdataPath  = "../../../testdata"
	manifestsPath = "../../../manifests"
	scriptsPath   = "../../../scripts"

	testFileName = "compositiondefinition-fireworksapp.yaml"
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	namespace = "demo-system"
	clusterName = "krateo"
	testenv = env.New()
	kindCluster := kind.NewCluster(clusterName)

	testenv.Setup(
		envfuncs.CreateCluster(kindCluster, clusterName),
		e2e.CreateNamespace(namespace),
		e2e.CreateNamespace("krateo-system"),

		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			r.WithNamespace(namespace)

			// Build the docker image
			if p := utils.RunCommand(
				fmt.Sprintf("docker build -t %s ../../..", "kind.local/core-provider:latest"),
			); p.Err() != nil {
				return ctx, p.Err()
			}

			err = kindCluster.LoadImage(ctx, "kind.local/core-provider:latest")
			if err != nil {
				return ctx, err
			}

			// uncomment to build and load the image in local testing
			// err = kindCluster.LoadImage(ctx, "kind.local/composition-dynamic-controller:latest")
			// if err != nil {
			// 	return ctx, err
			// }

			certificatesProc := utils.RunCommand("../../../scripts/reload.sh")
			if err := certificatesProc.Err(); err != nil {
				return ctx, err
			}

			// Install CRDs
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(crdPath)), "*.yaml",
				decoder.CreateIgnoreAlreadyExists(r),
			)

			time.Sleep(1 * time.Minute)

			return ctx, nil
		},
	).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.TeardownCRDs(crdPath, "core.krateo.io_compositiondefinitions.yaml"),
		envfuncs.DestroyCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}

func TestCreate(t *testing.T) {
	os.Setenv("DEBUG", "1")
	resource_ns := "krateo-system"

	f := features.New("Setup").
		Assess("Test Create", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fail()
			}
			apis.AddToScheme(r.GetScheme())
			r.WithNamespace(resource_ns)

			res := v1alpha1.CompositionDefinition{}

			err = decoder.DecodeFile(
				os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test")), testFileName,
				&res,
				decoder.MutateNamespace(resource_ns),
			)
			if err != nil {
				t.Fatal(err)
			}

			// Create CompositionDefinition
			err = r.Create(ctx, &res)
			if err != nil {
				t.Fatal(err)
			}

			err = wait.For(
				conditions.New(r).DeploymentAvailable("core-provider-dev", "demo-system"),
				wait.WithTimeout(15*time.Minute),
				wait.WithInterval(15*time.Second),
			)
			if err != nil {
				depl := appsv1.Deployment{}
				r.Get(ctx, res.Name, resource_ns, &depl)
				b, _ := json.MarshalIndent(depl, "", "  ")
				fmt.Println(string(b))
				t.Fatal(err)
			}

			//wait for resource to be created
			if err := wait.For(
				conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
					mg := object.(*v1alpha1.CompositionDefinition)
					return mg.GetCondition(rtv1.TypeReady).Reason == rtv1.ReasonAvailable
				}),
				wait.WithTimeout(15*time.Minute),
				wait.WithInterval(15*time.Second),
			); err != nil {
				obj := v1alpha1.CompositionDefinition{}
				r.Get(ctx, res.Name, resource_ns, &obj)
				b, _ := json.MarshalIndent(obj.Status, "", "  ")
				t.Logf("CompositionDefinition Status: %s", string(b))
				t.Fatal(err)
			}
			return ctx
		}).Assess("Test Patch Deployed Resource", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Fail()
		}
		apis.AddToScheme(r.GetScheme())
		r.WithNamespace(resource_ns)

		var res v1alpha1.CompositionDefinition
		err = decoder.DecodeFile(
			os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test")), testFileName,
			&res,
			decoder.MutateNamespace(resource_ns),
		)
		if err != nil {
			t.Fatal(err)
		}

		err = r.Get(ctx, res.Name, resource_ns, &res)
		if err != nil {
			t.Fatal(err)
		}

		oldDig := res.Status.Digest

		// Patch Deployment replica count
		var deployment appsv1.Deployment
		err = r.Get(ctx, "fireworksapps-v1-1-14-controller", "krateo-system", &deployment)
		if err != nil {
			t.Fatal(err)
		}
		deployment.Spec.Replicas = ptr.To(int32(2))
		err = r.Update(ctx, &deployment)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(1 * time.Minute)

		//wait for resource to be created
		if err := wait.For(
			conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
				mg := object.(*v1alpha1.CompositionDefinition)
				return mg.Status.Digest == oldDig
			}),
			wait.WithTimeout(15*time.Minute),
			wait.WithInterval(15*time.Second),
		); err != nil {
			obj := v1alpha1.CompositionDefinition{}
			r.Get(ctx, res.Name, resource_ns, &obj)
			b, _ := json.MarshalIndent(obj.Status, "", "  ")
			t.Logf("CompositionDefinition Status: %s", string(b))
			t.Fatal(err)
		}

		return ctx
	}).Assess("Test Change Version", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		const NewVersion = "1.1.15"
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Fail()
		}
		apis.AddToScheme(r.GetScheme())
		r.WithNamespace(resource_ns)

		var res v1alpha1.CompositionDefinition
		err = decoder.DecodeFile(
			os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test")), testFileName,
			&res,
			decoder.MutateNamespace(resource_ns),
		)
		if err != nil {
			t.Fatal(err)
		}

		err = r.Get(ctx, res.Name, resource_ns, &res)
		if err != nil {
			t.Fatal(err)
		}

		res.Spec.Chart.Version = NewVersion
		// Update CompositionDefinition
		err = r.Update(ctx, &res)
		if err != nil {
			t.Fatal(err)
		}

		//wait for resource to be created
		if err := wait.For(
			conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
				mg := object.(*v1alpha1.CompositionDefinition)
				return mg.GetCondition(rtv1.TypeReady).Reason == rtv1.ReasonAvailable &&
					len(mg.Status.Managed.VersionInfo) == 3 &&
					slices.ContainsFunc(mg.Status.Managed.VersionInfo, func(v v1alpha1.VersionDetail) bool {
						return v.Version == "v1-1-15"
					})
			}),
			wait.WithTimeout(15*time.Minute),
			wait.WithInterval(15*time.Second),
		); err != nil {
			obj := v1alpha1.CompositionDefinition{}
			r.Get(ctx, res.Name, resource_ns, &obj)
			b, _ := json.MarshalIndent(obj.Status, "", "  ")
			t.Logf("CompositionDefinition Status: %s", string(b))
			t.Fatal(err)
		}

		var crd apiextensionsv1.CustomResourceDefinition

		apiextensionsv1.AddToScheme(r.GetScheme())
		err = r.Get(ctx, "fireworksapps.composition.krateo.io", "", &crd)
		if err != nil {
			t.Fatal(err)
		}

		// Check CRD version
		if len(crd.Spec.Versions) != 3 {
			t.Fatalf("Expected 3 versions, got %d", len(crd.Spec.Versions))
		}
		if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
			return v.Name == "v1-1-15"
		}) {
			t.Fatalf("Expected version v1-1-15, got %v", crd.Spec.Versions)
		}
		if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
			return v.Name == "v1-1-14"
		}) {
			t.Fatalf("Expected version v1-1-14, got %v", crd.Spec.Versions)
		}
		if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
			return v.Name == "vacuum"
		}) {
			t.Fatalf("Expected version vacuum, got %v", crd.Spec.Versions)
		}

		return ctx
	}).Assess("Test Delete", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Fail()
		}
		apis.AddToScheme(r.GetScheme())
		r.WithNamespace(resource_ns)

		res := v1alpha1.CompositionDefinition{}

		err = r.Get(ctx, "fireworks-app-2", resource_ns, &res)
		if err != nil {
			t.Fatal(err)
		}

		// Delete CompositionDefinition
		err = r.Delete(ctx, &res)
		if err != nil {
			t.Fatal(err)
		}

		//wait for resource to be deleted
		if err := wait.For(
			conditions.New(r).ResourceDeleted(&res),
			wait.WithTimeout(15*time.Minute),
			wait.WithInterval(15*time.Second),
		); err != nil {
			t.Fatal(err)
		}
		return ctx
	}).
		Feature()

	testenv.Test(t, f)
}
