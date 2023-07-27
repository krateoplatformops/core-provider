//go:build integration
// +build integration

package tools

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	testChartUrl = "https://github.com/lucasepe/busybox-chart/releases/download/v0.2.0/dummy-chart-0.2.0.tgz"
)

func TestRoleForChartURL(t *testing.T) {
	role, err := RoleForChartURL(context.TODO(), testChartUrl)
	if err != nil {
		t.Fatal(err)
	}

	role.Name = "dummycharts-controller"
	role.Namespace = "krateo-system"
	role.CreationTimestamp = metav1.Now()

	dat, err := yaml.Marshal(role)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(dat))
}
