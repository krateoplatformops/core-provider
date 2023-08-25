package getter

import (
	"fmt"
	"testing"
)

func TestHelmGetter(t *testing.T) {
	g := &repoGetter{}
	dat, err := g.Get(GetOptions{
		URI:     "https://charts.bitnami.com/bitnami",
		Version: "12.8.3",
		Name:    "postgresql",
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(dat)
}
