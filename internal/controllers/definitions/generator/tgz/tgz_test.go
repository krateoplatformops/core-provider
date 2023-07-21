//go:build integration
// +build integration

package tgz

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"

	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/tgzfs"
)

func ExampleFetch() {
	const url = "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
	din, err := Fetch(context.TODO(), url)
	if err != nil {
		panic(err)
	}

	tfs, err := tgzfs.New(bytes.NewReader(din))
	if err != nil {
		panic(err)
	}

	fi, err := fs.Stat(tfs, "dummy-chart/values.schema.json")
	if err != nil {
		panic(err)
	}

	fmt.Println(fi.Name())
	fmt.Println(fi.IsDir())

	// Output:
	// values.schema.json
	// false
}
