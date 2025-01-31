//go:build integration
// +build integration

package compositiondefinitions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/krateoplatformops/core-provider/apis"
	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/snowplow/plumbing/e2e"
	xenv "github.com/krateoplatformops/snowplow/plumbing/env"
	"k8s.io/client-go/discovery"

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
	"sigs.k8s.io/e2e-framework/third_party/helm"
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

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), clusterName),
		envfuncs.LoadImageToCluster(clusterName, "ko.local/core-provider:latest"),                  // This is a local image built by ko. See scripts/build_local.sh
		envfuncs.LoadImageToCluster(clusterName, "ko.local/composition-dynamic-controller:latest"), // This is a local image built by ko. See scripts/build_local.sh
		envfuncs.SetupCRDs(crdPath, "core.krateo.io_compositiondefinitions.yaml"),
		e2e.CreateNamespace(namespace),
		e2e.CreateNamespace("krateo-system"),

		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			r.WithNamespace(namespace)

			certificatesProc := utils.RunCommand("../../../scripts/reload.sh")

			if err := certificatesProc.Err(); err != nil {
				return ctx, err
			}

			if err := wait.For(
				conditions.New(r).DeploymentAvailable("core-provider-dev", namespace),
				wait.WithTimeout(5*time.Minute),
				wait.WithInterval(15*time.Second),
			); err != nil {
				return ctx, err
			}

			// Install CRDs
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test/crds/argocd")), "*.yaml",
				decoder.CreateHandler(r),
			)
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test/crds/azuredevops-provider")), "*.yaml",
				decoder.CreateHandler(r),
			)
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test/crds/git-provider")), "*.yaml",
				decoder.CreateHandler(r),
			)
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test/crds/github-provider")), "*.yaml",
				decoder.CreateHandler(r),
			)
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test/crds/resourcetree")), "*.yaml",
				decoder.CreateHandler(r),
			)
			// Additional Krateo Setup
			// Add helm repos
			helmmgr := helm.New(cfg.KubeconfigFile())

			err = helmmgr.RunRepo(helm.WithArgs("add", "krateo", "https://charts.krateo.io"))
			if err != nil {
				return ctx, fmt.Errorf("Error adding helm repos: %v", err)
			}
			err = helmmgr.RunRepo(helm.WithArgs("update", "krateo"))
			if err != nil {
				return ctx, fmt.Errorf("Error adding helm repos: %v", err)
			}
			err = helmmgr.RunRepo(helm.WithArgs("add", "bitnami", "https://charts.bitnami.com/bitnami"))
			if err != nil {
				return ctx, fmt.Errorf("Error adding helm repos: %v", err)
			}
			err = helmmgr.RunRepo(helm.WithArgs("update", "bitnami"))
			if err != nil {
				return ctx, fmt.Errorf("Error adding helm repos: %v", err)
			}

			// Install etcd
			err = helmmgr.RunInstall(helm.WithReleaseName("bitnami/etcd"), helm.WithName("etcd"), helm.WithNamespace(namespace), helm.WithVersion("10.2.2"), helm.WithArgs("--set", "auth.rbac.create=false"))
			if err != nil {
				return ctx, fmt.Errorf("Error installing etcd: %v", err)
			}

			// Install backend
			err = helmmgr.RunInstall(helm.WithReleaseName("krateo/backend"), helm.WithName("backend"), helm.WithNamespace(namespace))
			if err != nil {
				return ctx, fmt.Errorf("Error installing backend: %v", err)
			}

			discoveryCli, err := discovery.NewDiscoveryClientForConfig(r.GetConfig())
			if err != nil {
				return ctx, fmt.Errorf("Error creating discovery client: %v", err)
			}

			wait.For(
				func(context.Context) (done bool, err error) {
					groups, err := discoveryCli.ServerGroups()
					if err != nil {
						return false, err
					}

					for _, group := range groups.Groups {
						if group.APIVersion == "templates.krateo.io/v1alpha1" {
							return true, nil
						}
					}

					return false, nil
				},
				wait.WithTimeout(5*time.Minute),
				wait.WithInterval(15*time.Second),
			)

			return ctx, nil
		},
	).Finish(
	// envfuncs.DeleteNamespace(namespace),
	// envfuncs.TeardownCRDs(crdPath, "core.krateo.io_compositiondefinitions.yaml"),
	// envfuncs.DestroyCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}

func TestCreate(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Setup").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			resource_ns := "krateo-system"
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

			//wait for resource to be created
			if err := wait.For(
				conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
					mg := object.(*v1alpha1.CompositionDefinition)
					return mg.GetCondition(rtv1.TypeReady).Reason == rtv1.ReasonAvailable
				}),
				wait.WithTimeout(4*time.Minute),
				wait.WithInterval(15*time.Second),
			); err != nil {
				obj := v1alpha1.CompositionDefinition{}
				r.Get(ctx, res.Name, resource_ns, &obj)
				b, _ := json.MarshalIndent(obj.Status, "", "  ")
				t.Logf("CompositionDefinition Status: %s", string(b))
				t.Fatal(err)
			}

			return ctx
		}).Feature()

	testenv.Test(t, f)
}

func TestDelete(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Setup").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			resource_ns := "krateo-system"
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

			//wait for resource to be created
			if err := wait.For(
				conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
					mg := object.(*v1alpha1.CompositionDefinition)
					return mg.GetCondition(rtv1.TypeSynced).Reason == rtv1.ReasonReconcileSuccess
				}),
				wait.WithTimeout(4*time.Minute),
				wait.WithInterval(15*time.Second),
			); err != nil {
				obj := v1alpha1.CompositionDefinition{}
				r.Get(ctx, res.Name, resource_ns, &obj)
				b, _ := json.MarshalIndent(obj.Status, "", "  ")
				t.Logf("CompositionDefinition Status: %s", string(b))
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
				wait.WithTimeout(4*time.Minute),
				wait.WithInterval(15*time.Second),
			); err != nil {
				t.Fatal(err)
			}

			return ctx
		}).Feature()

	testenv.Test(t, f)
}
