//go:build integration
// +build integration

package generator

import (
	"context"
	"fmt"
	"testing"
)

const (
	testChartUrl = "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
)

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
