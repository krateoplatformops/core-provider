package rbactools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestCreateServiceAccount(t *testing.T) {
	sa := types.NamespacedName{Name: "test-sa", Namespace: "test-namespace"}
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "serviceaccounts",
	}
	serviceaccount, err := CreateServiceAccount(gvr, sa, "testdata/serviceaccount_template.yaml", "serviceAccount", "test-sa")

	assert.NoError(t, err)

	expectedsa := v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gvr.Resource + "-" + gvr.Version,
			Namespace: sa.Namespace,
		},
	}
	assert.Equal(t, expectedsa.ObjectMeta.Name, serviceaccount.ObjectMeta.Name)
	assert.Equal(t, expectedsa.ObjectMeta.Namespace, serviceaccount.ObjectMeta.Namespace)
}
