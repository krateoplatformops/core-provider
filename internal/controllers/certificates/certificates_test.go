//go:build integration
// +build integration

package certificates

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/krateoplatformops/core-provider/apis"
	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/plumbing/e2e"
	xenv "github.com/krateoplatformops/plumbing/env"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testenv     env.Environment
	clusterName string
)

const (
	crdPath      = "../../../crds"
	namespace    = "test-system"
	templatePath = "testdata/manifests/mutating-webhook.yaml"
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	clusterName = "cert-manager-e2e"
	testenv = env.New()
	kindCluster := kind.NewCluster(clusterName)

	cleanAssetFolder := func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		_ = os.RemoveAll(filepath.Join(os.TempDir(), "assets"))
		return ctx, nil
	}

	testenv.Setup(
		cleanAssetFolder,
		envfuncs.CreateCluster(kindCluster, clusterName),
		e2e.CreateNamespace(namespace),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			// apply CRDs needed by the tests
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			if err := decoder.DecodeEachFile(ctx, os.DirFS(filepath.Join(crdPath)), "*.yaml", decoder.CreateIgnoreAlreadyExists(r)); err != nil {
				return ctx, err
			}
			// give apiserver a moment to register CRDs
			time.Sleep(2 * time.Second)
			return ctx, nil
		},
	).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.TeardownCRDs(crdPath, "core.krateo.io_compositiondefinitions.yaml"),
		envfuncs.DestroyCluster(clusterName),
		cleanAssetFolder,
	)

	os.Exit(testenv.Run(m))
}

func TestCertManagerE2E(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("CertManager E2E").
		Assess("Create CertManager and exercise methods", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			restCfg := cfg.Client().RESTConfig()
			// create controller-runtime client used by CertManager
			cli, err := client.New(restCfg, client.Options{})
			if err != nil {
				t.Fatalf("failed to create controller-runtime client: %v", err)
			}

			// ensure our APIs are in scheme for the fake client used by other helpers (optional)
			if err := apis.AddToScheme(cli.Scheme()); err != nil {
				t.Fatalf("failed to add apis to scheme: %v", err)
			}

			// build a real CertManager against the kind cluster
			opts := Opts{
				RestConfig:                  restCfg,
				MutatingWebhookTemplatePath: templatePath,
				WebhookServiceName:          "core-provider-webhook",
				WebhookServiceNamespace:     namespace,
				CertOpts: certs.GenerateClientCertAndKeyOpts{
					Duration:              time.Hour,
					LeaseExpirationMargin: 30 * time.Minute,
					Username:              "test",
					Approver:              "test",
				},
			}

			cm, err := NewCertManager(opts)
			if err != nil {
				t.Fatalf("failed to create CertManager: %v", err)
			}

			// call RefreshCertificates (this will generate certs and write into cm.certPath)
			if err := cm.RefreshCertificates(); err != nil {
				t.Fatalf("RefreshCertificates failed: %v", err)
			}

			// call ManageCertificates with a sample GVR (the CRD for compositiondefinitions must exist)
			gvr := schema.GroupVersionResource{
				Group:    "core.krateo.io",
				Version:  "v1alpha1",
				Resource: "compositiondefinitions",
			}
			if err := cm.ManageCertificates(context.Background(), gvr); err != nil {
				t.Fatalf("ManageCertificates failed: %v", err)
			}

			// UpdateExistingResources should not error (it will list compositiondefinitions in the cluster)
			if err := cm.UpdateExistingResources(context.Background()); err != nil {
				t.Fatalf("UpdateExistingResources failed: %v", err)
			}

			return ctx
		}).Feature()

	testenv.Test(t, f)
}
