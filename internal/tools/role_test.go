//go:build integration
// +build integration

package tools

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

const (
	testChartUrl = "https://github.com/krateoplatformops/postgresql-demo-chart/releases/download/12.8.3/postgresql-12.8.3.tgz"
	//testChartUrl  = "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
	testChartFile = "../../testdata/charts/postgresql-12.8.3.tgz"
)

func TestCreateRoleFromURL(t *testing.T) {
	pkg, err := chartfs.FromURL(testChartUrl)
	if err != nil {
		t.Fatal(err)
	}

	gvr, err := GroupVersionResource(pkg)
	if err != nil {
		t.Fatal(err)
	}

	role, err := CreateRole(pkg, gvr.Resource, types.NamespacedName{
		Name:      fmt.Sprintf("%s-controller", gvr.Resource),
		Namespace: "krateo-system",
	})
	if err != nil {
		t.Fatal(err)
	}

	dat, err := yaml.Marshal(role)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(dat))
}

func TestCreateRoleFromFile(t *testing.T) {
	pkg, err := chartfs.FromFile(testChartFile)
	if err != nil {
		t.Fatal(err)
	}

	gvr, err := GroupVersionResource(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if want := "composition.krateo.io/v12-8-3, Resource=postgresqls"; !cmp.Equal(gvr.String(), want) {
		t.Fatalf("got: %s, want: %s", gvr, want)
	}

	role, err := CreateRole(pkg, gvr.Resource, types.NamespacedName{
		Name:      fmt.Sprintf("%s-controller", gvr.Resource),
		Namespace: "krateo-system",
	})
	if err != nil {
		t.Fatal(err)
	}

	dat, err := yaml.Marshal(role)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(dat))
}
