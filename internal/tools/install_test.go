//go:build integration
// +build integration

package tools

import (
	"context"
	"os"
	"testing"

	"github.com/krateoplatformops/core-provider/internal/controllers/compositions/templates"
)

func Test_InstallCRD(t *testing.T) {
	dat, err := os.ReadFile(testCRD)
	if err != nil {
		t.Fatal(err)
	}

	crd, err := UnmarshalCRD(dat)
	if err != nil {
		t.Fatal(err)
	}

	cli := setupKubeClient(t)
	err = InstallCRD(context.Background(), cli, crd)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_InstallServiceAccount(t *testing.T) {
	values := templates.Values(templates.Renderoptions{
		Group:     "core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "krateo-system",
	})
	dat, err := templates.Render(templates.ServiceAccount, values)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := UnmarshalServiceAccount(dat)
	if err != nil {
		t.Fatal(err)
	}

	cli := setupKubeClient(t)
	err = InstallServiceAccount(context.Background(), cli, obj)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_InstallRole(t *testing.T) {
	values := templates.Values(templates.Renderoptions{
		Group:     "core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "krateo-system",
	})
	dat, err := templates.Render(templates.Role, values)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := UnmarshalRole(dat)
	if err != nil {
		t.Fatal(err)
	}

	cli := setupKubeClient(t)
	err = InstallRole(context.Background(), cli, obj)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_InstallRoleBinding(t *testing.T) {
	values := templates.Values(templates.Renderoptions{
		Group:     "core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "krateo-system",
	})
	dat, err := templates.Render(templates.RoleBinding, values)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := UnmarshalRoleBinding(dat)
	if err != nil {
		t.Fatal(err)
	}

	cli := setupKubeClient(t)
	err = InstallRoleBinding(context.Background(), cli, obj)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_InstallDeployment(t *testing.T) {
	values := templates.Values(templates.Renderoptions{
		Group:     "core.krateo.io",
		Version:   "v0-2-0",
		Resource:  "dummycharts",
		Namespace: "krateo-system",
	})
	dat, err := templates.Render(templates.Deployment, values)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := UnmarshalDeployment(dat)
	if err != nil {
		t.Fatal(err)
	}

	cli := setupKubeClient(t)
	err = InstallDeployment(context.Background(), cli, obj)
	if err != nil {
		t.Fatal(err)
	}
}
