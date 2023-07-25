//go:build integration
// +build integration

package tools

import (
	"fmt"
	"os"
	"testing"
)

func Test_unmarshalCRD(t *testing.T) {
	dat, err := os.ReadFile(testCRD)
	if err != nil {
		t.Fatal(err)
	}

	crd, err := UnmarshalCRD(dat)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%v\n", crd.Spec.Names.Categories)
}
