//go:build integration
// +build integration

package generator

import (
	"context"
	"testing"
)

func TestGroupVersionKindFromTarGzipURL(t *testing.T) {
	url := "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
	gvk, err := GroupVersionKindFromTarGzipURL(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf(gvk.String())

	gvr := ToGroupVersionResource(gvk)

	t.Logf(gvr.String())
}
