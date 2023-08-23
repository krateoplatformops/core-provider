//go:build integration
// +build integration

package generator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools/getter"
)

const (
	testChartUrl  = "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
	testChartFile = "../../../../testdata/charts/postgresql-12.8.3.tgz"
)

func TestGeneratorFromURL(t *testing.T) {
	buf, err := getter.NewHTTPGetter().Get(testChartUrl)
	if err != nil {
		t.Fatal(err)
	}

	gen, err := ForData(context.Background(), buf)
	if err != nil {
		t.Fatal(err)
	}

	dat, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

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

	gen, err := ForData(context.Background(), bytes.NewBuffer(all))
	if err != nil {
		t.Fatal(err)
	}

	dat, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(dat))
}
