package templates

import (
	_ "embed"
	"fmt"
	"testing"
)

func TestDeploymentManifest(t *testing.T) {
	values := Values(Renderoptions{
		Group:     "dummy-charts.core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "default",
	})
	bin, err := RenderDeployment(values)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(bin))
}
