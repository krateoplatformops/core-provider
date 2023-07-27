package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/text"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgz"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgzfs"
	"gopkg.in/yaml.v2"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func RoleForChartURL(ctx context.Context, url string) (*rbacv1.Role, error) {
	bin, err := tgz.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}

	pkg, err := tgzfs.New(bytes.NewReader(bin))
	if err != nil {
		return nil, err
	}

	all, err := fs.ReadDir(pkg, ".")
	if err != nil {
		return nil, err
	}

	if len(all) != 1 {
		return nil, fmt.Errorf("archive '%s' should contain only one root dir", url)
	}

	rootDir := all[0].Name()
	entries, err := fs.ReadDir(pkg, path.Join(rootDir, "templates"))
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("empty 'templates' folder")
	}

	resource, err := deriveResourceName(pkg, rootDir)
	if err != nil {
		return nil, err
	}

	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
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

	pols := map[string][]string{}
	for _, el := range entries {
		if strings.HasPrefix(el.Name(), "_") {
			continue
		}

		nfo, err := createPolicyInfo(pkg, path.Join(rootDir, "templates", el.Name()))
		if err != nil {
			return nil, err
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

func deriveResourceName(fs fs.FS, rootDir string) (string, error) {
	fin, err := fs.Open(rootDir + "/Chart.yaml")
	if err != nil {
		return "", err
	}
	defer fin.Close()

	din, err := io.ReadAll(fin)
	if err != nil {
		return "", err
	}

	res := map[string]any{}
	if err := yaml.Unmarshal(din, &res); err != nil {
		return "", err
	}

	name := res["name"].(string)
	kind := flect.Pascalize(text.ToGolangName(name))
	resource := strings.ToLower(flect.Pluralize(kind))
	return resource, nil
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
