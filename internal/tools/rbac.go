package tools

import (
	"bufio"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func populateFromFile(discovery discovery.DiscoveryInterface, fi fs.File, crdInfoList []CRDInfo, role *rbacv1.Role, clusterRole *rbacv1.ClusterRole) error {
	findKindAPIVersion := func(fi fs.File) (policies []metav1.TypeMeta, errs []error) {
		scanner := bufio.NewScanner(fi)
		kind, apiVersion := "", ""
		for scanner.Scan() {
			kindReg := regexp.MustCompile(`^kind:\s*`)
			apiVersionReg := regexp.MustCompile(`^apiVersion:\s*`)
			if apiVersionReg.MatchString(scanner.Text()) {
				apiVersion = strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "apiVersion:"))
			}
			if kindReg.MatchString(scanner.Text()) {
				kind = strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "kind:"))
			}
			if kind != "" && apiVersion != "" {
				invalidKindAPIVersionReg := regexp.MustCompile(`{{(?:[^{}]*|{{.*?}})*}}`)
				if invalidKindAPIVersionReg.MatchString(kind) || invalidKindAPIVersionReg.MatchString(apiVersion) {
					errs = append(errs, fmt.Errorf("kind or apiVersion contains template: %s, %s", kind, apiVersion))
					kind, apiVersion = "", ""
					continue
				}

				policies = append(policies, metav1.TypeMeta{
					Kind:       kind,
					APIVersion: apiVersion,
				})
				kind, apiVersion = "", ""
			}
		}
		return
	}

	policies, errs := findKindAPIVersion(fi)
	if len(policies) == 0 {
		if len(errs) > 0 {
			wrappedErrs := ErrKindApiVersion
			for _, err := range errs {
				wrappedErrs = fmt.Errorf("%w - %w", wrappedErrs, err)
			}
			return wrappedErrs
		}
		return ErrKindApiVersion
	}

	for _, policy := range policies {
		gv, err := schema.ParseGroupVersion(policy.APIVersion)
		if err != nil {
			return fmt.Errorf("failed to parse group version: %w", err)
		}
		gvr := schema.GroupVersionResource{
			Group:    gv.Group,
			Version:  gv.Version,
			Resource: strings.ToLower(flect.Pluralize(policy.Kind)),
		}
		namespaced, err := isGRNamespaced(discovery, schema.GroupResource{
			Group:    gvr.Group,
			Resource: gvr.Resource,
		}, crdInfoList)
		if err != nil {
			//Default to clusterRole
			namespaced = false
		}

		policyRule := rbacv1.PolicyRule{
			APIGroups: []string{gvr.Group},
			Resources: []string{gvr.Resource, fmt.Sprintf("%s/status", gvr.Resource)},
			Verbs:     []string{"*"},
		}

		if namespaced {
			role.Rules = append(role.Rules, policyRule)
		} else {
			clusterRole.Rules = append(clusterRole.Rules, policyRule)
		}
	}

	if len(errs) > 0 {
		wrappedErrs := ErrKindApiVersion
		for _, err := range errs {
			wrappedErrs = fmt.Errorf("%w - %w", wrappedErrs, err)
		}
		return wrappedErrs
	}
	return nil
}
func populateFromDir(pkg *chartfs.ChartFS, templatesDir string, dir []fs.DirEntry, crdInfoList []CRDInfo, discovery discovery.DiscoveryInterface, role *rbacv1.Role, clusterRole *rbacv1.ClusterRole) error {
	for _, file := range dir {
		if file.IsDir() {
			subdir := path.Join(templatesDir, file.Name())
			dir, err := fs.ReadDir(pkg.FS(), subdir)
			if err != nil {
				return fmt.Errorf("failed to read directory %s: %w", templatesDir, err)
			}
			if err := populateFromDir(pkg, subdir, dir, crdInfoList, discovery, role, clusterRole); err != nil {
				return err
			}
		}
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			fi, err := pkg.Open(path.Join(templatesDir, file.Name()))
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", file.Name(), err)
			}
			if fi == nil {
				continue
			}
			defer fi.Close()
			err = populateFromFile(discovery, fi, crdInfoList, role, clusterRole)
			if err != nil {
				return fmt.Errorf("%w - folder: %s - file: %s", err, templatesDir, file.Name())
			}
		}
	}
	return nil
}

func PopulateRoleClusterRole(pkg *chartfs.ChartFS, discovery discovery.DiscoveryInterface, role *rbacv1.Role, clusterRole *rbacv1.ClusterRole) error {
	crdInfoList, _ := GetCRDInfoList(pkg)

	templatesDir := path.Join(pkg.RootDir(), "templates")
	dir, err := fs.ReadDir(pkg.FS(), templatesDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", templatesDir, err)
	}
	grList := [...]schema.GroupResource{
		{
			Resource: "namespaces",
		},
		{
			Resource: "secrets",
		},
	}
	for _, gr := range grList {
		namespaced, err := isGRNamespaced(discovery, gr, crdInfoList)
		if err != nil {
			//Default to clusterRole
			namespaced = false
		}
		rbac := rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{gr.Resource},
			Verbs:     []string{"*"},
		}
		if namespaced {
			role.Rules = append(role.Rules, rbac)
		} else {
			clusterRole.Rules = append(clusterRole.Rules, rbac)
		}
	}

	err = populateFromDir(pkg, templatesDir, dir, crdInfoList, discovery, role, clusterRole)
	if err != nil {
		return fmt.Errorf("failed to populate role and clusterRole: %w", err)
	}

	return nil
}

func isGRNamespaced(discovery discovery.DiscoveryInterface, gr schema.GroupResource, crdList []CRDInfo) (bool, error) {
	groupList, resLists, err := discovery.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}

	for _, crd := range crdList {
		if crd.GroupVersionKind.Group == gr.Group && crd.GroupVersionKind.Kind == gr.Resource {
			return crd.Namespaced, nil
		}
	}

	for _, group := range groupList {
		if group.Name == gr.Group {
			for _, res := range resLists {
				for _, res := range res.APIResources {
					resource := strings.ToLower(flect.Pluralize(res.Kind))
					if resource == gr.Resource {
						return res.Namespaced, nil
					}
				}
			}
		}
	}

	return false, fmt.Errorf("resource %s not found", gr.String())

}
