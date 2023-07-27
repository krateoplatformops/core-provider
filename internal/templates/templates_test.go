package templates

import (
	_ "embed"
	"fmt"
	"testing"
)

func TestServiceAccountManifest(t *testing.T) {
	values := Values(Renderoptions{
		Group:     "dummy-charts.core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "default",
	})

	bin, err := Render(ServiceAccount, values)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(bin))
}

func TestRoleManifest(t *testing.T) {
	values := Values(Renderoptions{
		Group:     "composition.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "krateo-system",
	})
	bin, err := Render(Role, values)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(bin))
}

func TestRoleBindingManifest(t *testing.T) {
	values := Values(Renderoptions{
		Group:     "dummy-charts.core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "default",
	})
	bin, err := Render(RoleBinding, values)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(bin))
}

func TestDeploymentManifest(t *testing.T) {
	values := Values(Renderoptions{
		Group:     "dummy-charts.core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "default",
	})
	bin, err := Render(Deployment, values)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(bin))
}
