//go:build integration
// +build integration

package generator

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/helm/getter"
)

const (
	testChartFile        = "../../../../testdata/charts/postgresql-12.8.3.tgz"
	testValuesSchemaFile = "../../../../testdata/values.schema.json"
)

func TestGeneratorTGZ(t *testing.T) {
	buf, _, err := getter.Get(getter.GetOptions{
		URI: "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz",
	})
	if err != nil {
		t.Fatal(err)
	}

	pkg, dir, err := ChartInfoFromBytes(context.TODO(), buf)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := Generate(context.TODO(), dir,
		ChartGroupVersionKindGetter(pkg, dir),
		ChartValuesSchemaGetter(pkg, dir))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(dat))
}

func TestGeneratorTGZIssues(t *testing.T) {
	const (
		uri = "https://github.com/krateoplatformops/krateo-v2-template-fireworksapp/releases/download/0.0.1/fireworks-app-0.1.0.tgz"
	)
	buf, _, err := getter.Get(getter.GetOptions{
		URI: uri,
	})
	if err != nil {
		t.Fatal(err)
	}

	//os.Setenv("GEN_CLEAN_WORKDIR", "NO")
	pkg, dir, err := ChartInfoFromBytes(context.TODO(), buf)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := Generate(context.TODO(), dir,
		ChartGroupVersionKindGetter(pkg, dir),
		ChartValuesSchemaGetter(pkg, dir))

	fmt.Println(string(dat))
}

func TestGeneratorOCI(t *testing.T) {
	buf, _, err := getter.Get(getter.GetOptions{
		URI:     "oci://registry-1.docker.io/bitnamicharts/postgresql",
		Version: "12.8.3",
		Repo:    "",
	})
	if err != nil {
		t.Fatal(err)
	}

	pkg, dir, err := ChartInfoFromBytes(context.TODO(), buf)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := Generate(context.TODO(), dir,
		ChartGroupVersionKindGetter(pkg, dir),
		ChartValuesSchemaGetter(pkg, dir))

	fmt.Println(string(dat))
}

func TestGeneratorREPO(t *testing.T) {
	buf, url, err := getter.Get(getter.GetOptions{
		URI:     "https://charts.bitnami.com/bitnami",
		Version: "12.8.3",
		Repo:    "postgresql",
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(url)

	pkg, dir, err := ChartInfoFromBytes(context.TODO(), buf)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := Generate(context.TODO(), dir,
		ChartGroupVersionKindGetter(pkg, dir),
		ChartValuesSchemaGetter(pkg, dir))

	fmt.Println(string(dat))
}

func TestGeneratorFromFile(t *testing.T) {
	fin, err := os.Open(testChartFile)
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	all, err := io.ReadAll(fin)
	if err != nil {
		t.Fatal(err)
	}

	pkg, dir, err := ChartInfoFromBytes(context.TODO(), all)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := Generate(context.TODO(), dir,
		ChartGroupVersionKindGetter(pkg, dir),
		ChartValuesSchemaGetter(pkg, dir))

	fmt.Println(string(dat))
}

func TestGeneratorStream(t *testing.T) {
	fin, err := os.Open(testValuesSchemaFile)
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	//os.Setenv("GEN_CLEAN_WORKDIR", "NO")
	dat, err := Generate(context.TODO(), "fake",
		FakeGroupVersionKindGetter("Test"),
		StreamValuesSchemaGetter(fin))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(dat))
}
