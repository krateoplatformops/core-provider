//go:build integration
// +build integration

package generator

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testChartUrl = "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
)

func TestGroupVersionKindFromTarGzipURL(t *testing.T) {
	expGVK := schema.FromAPIVersionAndKind("composition.krateo.io/v0-2-0", "DummyChart")
	gotGVK, err := GroupVersionKindFromTarGzipURL(context.Background(), testChartUrl)
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(expGVK, gotGVK) {
		t.Fatalf("invalid GVK - expected: %s, got: %s", expGVK.String(), gotGVK.String())
	}

	expGVR := schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v0-2-0",
		Resource: "dummycharts",
	}
	gotGVR := ToGroupVersionResource(expGVK)
	if !cmp.Equal(expGVR, gotGVR) {
		t.Fatalf("invalid GVR - expected: %s, got: %s", expGVR.String(), gotGVR.String())
	}
}

func TestGenerator(t *testing.T) {
	gen, err := ForTarGzipURL(context.Background(), testChartUrl)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(dat))
}
