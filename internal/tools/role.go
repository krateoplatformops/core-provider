package tools

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/avast/retry-go"
	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func CreateRole(pkg *chartfs.ChartFS, resource string, opts types.NamespacedName) (rbacv1.Role, error) {
	entries, err := fs.ReadDir(pkg, path.Join(pkg.RootDir(), "templates"))
	if err != nil {
		return rbacv1.Role{}, err
	}

	if len(entries) == 0 {
		return rbacv1.Role{}, fmt.Errorf("empty 'templates' folder in chart")
	}

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
				Resources: []string{"definitions", "definitions/status"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"composition.krateo.io"},
				Resources: []string{resource, fmt.Sprintf("%s/status", resource)},
				Verbs:     []string{"*"},
			},
		},
	}

	pols := map[string][]string{
		"": {"secrets"},
	}

	for _, el := range entries {
		if strings.HasPrefix(el.Name(), "_") {
			continue
		}

		nfo, err := createPolicyInfo(pkg, path.Join(pkg.RootDir(), "templates", el.Name()))
		if err != nil {
			return rbacv1.Role{}, err
		}

		lst, ok := pols[nfo.group]
		if ok {
			lst = append(lst, nfo.resource)
			pols[nfo.group] = lst
		} else {
			pols[nfo.group] = []string{nfo.resource}
		}
	}

	for grp, res := range pols {
		role.Rules = append(role.Rules, rbacv1.PolicyRule{
			APIGroups: []string{grp},
			Resources: res,
			Verbs:     []string{"*"},
		})
	}

	return role, nil
}

func createPolicyInfo(fs fs.FS, filename string) (nfo policytInfo, err error) {
	fin, err := fs.Open(filename)
	if err != nil {
		return nfo, err
	}
	defer fin.Close()

	scanner := bufio.NewScanner(fin)

	groupOK := false
	resourceOK := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "apiGroup") {
			idx := strings.IndexRune(line, ':')
			if idx != -1 {
				gv, err := schema.ParseGroupVersion(strings.TrimSpace(line[idx+1:]))
				if err != nil {
					return nfo, err
				}
				nfo.group = gv.Group
				groupOK = true
			}
		}

		if strings.HasPrefix(line, "kind") {
			idx := strings.IndexRune(line, ':')
			if idx != -1 {
				kind := strings.TrimSpace(line[idx+1:])
				nfo.resource = strings.ToLower(flect.Pluralize(kind))
				resourceOK = true
			}
		}

		if groupOK && resourceOK {
			break // don't read all yaml
		}
	}

	err = scanner.Err()
	return nfo, err
}

type policytInfo struct {
	group    string
	resource string
}
