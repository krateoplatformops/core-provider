package getter

import (
	"fmt"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
)

func TestOCIGetter(t *testing.T) {
	g, err := NewOCIGetter()
	if err != nil {
		t.Fatal(err)
	}

	buf, err := g.Get("oci://registry-1.docker.io/bitnamicharts/postgresql")
	if err != nil {
		t.Fatal(err)
	}

	cfs, err := chartfs.FromReader(buf)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(cfs.RootDir())

}

func TestHTTPGetter(t *testing.T) {
	g := NewHTTPGetter()

	buf, err := g.Get("https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz")
	if err != nil {
		t.Fatal(err)
	}

	cfs, err := chartfs.FromReader(buf)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(cfs.RootDir())

}
