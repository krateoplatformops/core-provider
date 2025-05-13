//go:build integration
// +build integration

package chart

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/crdgen"
	"github.com/krateoplatformops/plumbing/e2e"
	xenv "github.com/krateoplatformops/plumbing/env"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/decoder"
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
	crdPath       = "../../../../crds"
	testdataPath  = "../../../../testdata"
	manifestsPath = "../../../../manifests"
	scriptsPath   = "../../../../scripts"

	testFileName = "compositiondefinition-common.yaml"
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	namespace = "demo-system"
	clusterName = "krateo"
	testenv = env.New()

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), clusterName),
		envfuncs.SetupCRDs(crdPath, "core.krateo.io_compositiondefinitions.yaml"),
		e2e.CreateNamespace(namespace),
		e2e.CreateNamespace("krateo-system"),

		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			r.WithNamespace(namespace)

			// Install CRDs
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(testdataPath, "compositiondefinitions_test/crds/finops")), "*.yaml",
				decoder.CreateHandler(r),
			)
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

			return ctx, nil
		},
	).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}

func TestJsonSchemaFromOCI(t *testing.T) {

	os.Setenv("DEBUG", "1")

	f := features.New("Setup").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			return ctx
		}).Assess("Lookup", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		nfo := v1alpha1.ChartInfo{
			Url:     "oci://registry-1.docker.io/bitnamicharts/redis",
			Version: "18.0.1",
		}

		cli, err := client.New(cfg.Client().RESTConfig(), client.Options{})
		if err != nil {
			t.Fatal(err)
		}

		pkg, rootdir, err := ChartInfoFromSpec(context.TODO(), cli, &nfo)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("root dir: %s\n", rootdir)

		s, err := ChartJsonSchemaGetter(pkg, rootdir).Get()
		if err != nil {
			t.Fatal(err)
		}

		fmt.Println(string(s))
		return ctx
	}).Feature()

	testenv.Test(t, f)

}

func TestCRDGenFromOCI(t *testing.T) {
	nfo := v1alpha1.ChartInfo{
		Url:     "oci://registry-1.docker.io/bitnamicharts/redis",
		Version: "18.0.1",
	}

	pkg, dir, err := ChartInfoFromSpec(context.TODO(), nil, &nfo)
	if err != nil {
		t.Fatal(err)
	}

	gvk, err := ChartGroupVersionKind(pkg, dir)
	if err != nil {
		t.Fatal(err)
	}

	res := crdgen.Generate(context.TODO(), crdgen.Options{
		Managed:              true,
		WorkDir:              dir,
		GVK:                  gvk,
		Categories:           []string{"compositions", "comps"},
		SpecJsonSchemaGetter: ChartJsonSchemaGetter(pkg, dir),
	})
	if res.Err != nil {
		t.Fatal(res.Err)
	}

	fmt.Println(string(res.Manifest))
}
