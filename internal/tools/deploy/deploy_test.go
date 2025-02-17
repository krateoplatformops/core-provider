//go:build integration
// +build integration

package deploy

import (
	"context"
	"os"
	"testing"

	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/snowplow/plumbing/e2e"
	xenv "github.com/krateoplatformops/snowplow/plumbing/env"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
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

	testFileName = "compositiondefinition-common.yaml"
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	namespace = "demo-system"
	clusterName = "krateo"
	testenv = env.New()

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), clusterName),
		e2e.CreateNamespace(namespace),
		e2e.CreateNamespace("krateo-system"),

		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			r.WithNamespace(namespace)

			return ctx, nil
		},
	).Finish(
	// envfuncs.DeleteNamespace(namespace),
	// envfuncs.DestroyCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}

func TestDeploy(t *testing.T) {
	os.Setenv("DEBUG", "1")

	f := features.New("Setup").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		}).Assess("Deploy", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		cli, err := client.New(cfg.Client().RESTConfig(), client.Options{})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
			return ctx
		}
		opts := DeployOptions{
			DiscoveryClient:        memory.NewMemCacheClient(discovery.NewDiscoveryClientForConfigOrDie(cfg.Client().RESTConfig())),
			RBACFolderPath:         "testdata",
			DeploymentTemplatePath: "testdata/deploy.yaml",
			ConfigmapTemplatePath:  "testdata/cm.yaml",
			KubeClient:             cli,
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "test-deploy",
			},
			Spec: &v1alpha1.ChartInfo{
				InsecureSkipVerifyTLS: true,
				Version:               "1.1.10",
				Repo:                  "fireworks-app",
				Url:                   "https://charts.krateo.io",
				Credentials: &v1alpha1.Credentials{
					Username: "admin",
					PasswordRef: rtv1.SecretKeySelector{
						Key: "password",
						Reference: rtv1.Reference{
							Name:      "test-secret",
							Namespace: "default",
						},
					},
				},
			},
			Log: func(msg string, keysAndValues ...any) {},
		}

		err, rbacErr := Deploy(context.Background(), schema.GroupVersionResource{
			Group:    "compositions.krateo.io",
			Version:  "v1alpha1",
			Resource: "fireworksapps",
		}, cli, opts)
		assert.NoError(t, err)
		assert.NoError(t, rbacErr)

		return ctx
	}).Feature()

	testenv.Test(t, f)
}
