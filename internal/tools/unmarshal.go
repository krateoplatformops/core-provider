package tools

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeploymentExists(ctx context.Context, kube client.Client, obj client.Object) (bool, error) {
	tmp := appsv1.Deployment{}
	err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

func UnmarshalDeployment(dat []byte) (*appsv1.Deployment, error) {
	if !clientsetscheme.Scheme.IsGroupRegistered("apps") {
		_ = appsv1.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &appsv1.Deployment{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

func UnmarshalServiceAccount(dat []byte) (*corev1.ServiceAccount, error) {
	if !clientsetscheme.Scheme.IsGroupRegistered("") {
		_ = corev1.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &corev1.ServiceAccount{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

func UnmarshalRole(dat []byte) (*rbacv1.Role, error) {
	if !clientsetscheme.Scheme.IsGroupRegistered("rbac.authorization.k8s.io") {
		_ = rbacv1.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &rbacv1.Role{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

func UnmarshalRoleBinding(dat []byte) (*rbacv1.RoleBinding, error) {
	if !clientsetscheme.Scheme.IsGroupRegistered("rbac.authorization.k8s.io") {
		_ = rbacv1.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &rbacv1.RoleBinding{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

func UnmarshalCRD(dat []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
	if !clientsetscheme.Scheme.IsGroupRegistered("apiextensions.k8s.io") {
		_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &apiextensionsv1.CustomResourceDefinition{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}
