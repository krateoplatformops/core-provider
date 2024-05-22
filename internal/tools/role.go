package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/avast/retry-go"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"

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

func InitRole(pkg *chartfs.ChartFS, resource string, opts types.NamespacedName) (rbacv1.Role, error) {

	role := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.krateo.io"},
				Resources: []string{"compositiondefinitions", "compositiondefinitions/status"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"composition.krateo.io"},
				Resources: []string{resource, fmt.Sprintf("%s/status", resource)},
				Verbs:     []string{"*"},
			},
		},
	}

	return role, nil
}

// func createPolicyInfo(dec *utilyaml.YAMLReader) (nfo policytInfo, err error) {
// 	buf, err := dec.Read()
// 	if err != nil {
// 		return nfo, err
// 	}

// 	tm := metav1.TypeMeta{}
// 	if err := yaml.Unmarshal(buf, &tm); err != nil {
// 		return nfo, err
// 	}
// 	if tm.Kind == "" {
// 		return nfo, err
// 	}

// 	gv, err := schema.ParseGroupVersion(tm.APIVersion)
// 	if err != nil {
// 		return nfo, err
// 	}

// 	nfo.group = gv.Group
// 	nfo.resource = strings.ToLower(flect.Pluralize(tm.Kind))
// 	return nfo, err
// }

// type policytInfo struct {
// 	group    string
// 	resource string
// }
