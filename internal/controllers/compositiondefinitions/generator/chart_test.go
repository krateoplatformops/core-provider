//go:build integration
// +build integration

package generator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/generator"
	"github.com/krateoplatformops/crdgen"
)

func TestJsonSchemaFromOCI(t *testing.T) {
	nfo := v1alpha1.ChartInfo{
		Url:     "oci://registry-1.docker.io/bitnamicharts/redis",
		Version: "18.0.1",
	}

	pkg, rootdir, err := generator.ChartInfoFromSpec(context.TODO(), &nfo)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("root dir: %s\n", rootdir)

	s, err := generator.ChartJsonSchemaGetter(pkg, rootdir).Get()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(s))
}

func TestCRDGenFromOCI(t *testing.T) {
	nfo := v1alpha1.ChartInfo{
		Url:     "oci://registry-1.docker.io/bitnamicharts/redis",
		Version: "18.0.1",
	}

	pkg, dir, err := generator.ChartInfoFromSpec(context.TODO(), &nfo)
	if err != nil {
		t.Fatal(err)
	}

	gvk, err := generator.ChartGroupVersionKind(pkg, dir)
	if err != nil {
		t.Fatal(err)
	}

	res := crdgen.Generate(context.TODO(), crdgen.Options{
		Managed:              true,
		WorkDir:              dir,
		GVK:                  gvk,
		Categories:           []string{"compositions", "comps"},
		SpecJsonSchemaGetter: generator.ChartJsonSchemaGetter(pkg, dir),
	})
	if res.Err != nil {
		t.Fatal(res.Err)
	}

	fmt.Println(string(res.Manifest))
}
