package rbactools

import (
	"context"
	"errors"
	"strings"

	"github.com/avast/retry-go"
	"github.com/gobuffalo/flect"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const strErrKindApiVersion = "failed to find kind and apiVersion"

var ErrKindApiVersion = errors.New(strErrKindApiVersion)

func UninstallRole(ctx context.Context, opts UninstallOptions) error {
	return retry.Do(
		func() error {
			obj := rbacv1.Role{}
			err := opts.KubeClient.Get(ctx, opts.NamespacedName, &obj, &client.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			err = opts.KubeClient.Delete(ctx, &obj, &client.DeleteOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			if opts.Log != nil {
				opts.Log("Role successfully uninstalled",
					"name", obj.GetName(), "namespace", obj.GetNamespace())
			}

			return nil
		},
	)
}

func InstallRole(ctx context.Context, kube client.Client, obj *rbacv1.Role) error {
	return retry.Do(
		func() error {
			tmp := rbacv1.Role{}
			err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return kube.Create(ctx, obj)
				}

				return err
			}

			return nil
		},
	)
}

func InitRole(resource string, opts types.NamespacedName) rbacv1.Role {
	kind := strings.ToLower(flect.Singularize(resource))
	role := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "role",
				"app.kubernetes.io/instance":   "manager-role",
				"app.kubernetes.io/component":  "rbac",
				"app.kubernetes.io/created-by": kind,
				"app.kubernetes.io/part-of":    kind,
				"app.kubernetes.io/managed-by": "kustomize",
			},
		},
		Rules: []rbacv1.PolicyRule{},
	}

	return role
}
