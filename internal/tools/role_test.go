//go:build integration
// +build integration

package tools_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	testChartUrl = "https://github.com/krateoplatformops/postgresql-demo-chart/releases/download/12.8.3/postgresql-12.8.3.tgz"
)

func TestInstallRole(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	obj, err := createRoleFromURL()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.InstallRole(context.TODO(), kube, &obj)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallRole(t *testing.T) {
	kube, err := setupKubeClient()
	if err != nil {
		t.Fatal(err)
	}

	err = tools.UninstallRole(context.TODO(), tools.UninstallOptions{
		KubeClient: kube,
		NamespacedName: types.NamespacedName{
			Name:      "postgresqls-controller",
			Namespace: "default",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateRoleFromTGZ(t *testing.T) {
	obj, err := createRoleFromURL()
	if err != nil {
		t.Fatal(err)
	}

	dat, err := yaml.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(dat))
}

func createRoleFromURL() (rbacv1.Role, error) {
	home, err := os.UserHomeDir()
	cfg, err := clientcmd.BuildConfigFromFlags("", path.Join(home, ".kube/config"))
	if err != nil {
		return rbacv1.Role{}, err
	}
	cli, err := client.New(cfg, client.Options{})
	if err != nil {
		return rbacv1.Role{}, err
	}
	pkg, err := chartfs.ForSpec(context.TODO(), cli, &v1alpha1.ChartInfo{
		Url: testChartUrl,
	})
	if err != nil {
		return rbacv1.Role{}, err
	}

	gvk, err := tools.GroupVersionKind(pkg)
	if err != nil {
		return rbacv1.Role{}, err
	}

	gvr := tools.ToGroupVersionResource(gvk)

	return tools.InitRole(pkg, gvr.Resource, types.NamespacedName{
		Name:      fmt.Sprintf("%s-controller", gvr.Resource),
		Namespace: "default",
	})
}
