package rbacgen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	crdinfo "github.com/krateoplatformops/core-provider/internal/tools/crdInfo"
	"github.com/krateoplatformops/core-provider/internal/tools/rbactools"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const strErrKindApiVersion = "failed to find kind and apiVersion"

var ErrKindApiVersion = errors.New(strErrKindApiVersion)

type Resource struct {
	Kind                string
	APIVersion          string
	Namespace           string
	IsNamespaceTemlated bool
	IsNamespaced        bool
}

type RbacGenerator struct {
	discovery       discovery.DiscoveryInterface
	pkg             *chartfs.ChartFS
	deployName      string
	deployNamespace string
	secretNamespace string
	secretName      string
}

type RBAC struct {
	Role               *rbacv1.Role
	ClusterRole        *rbacv1.ClusterRole
	RoleBinding        *rbacv1.RoleBinding
	ClusterRoleBinding *rbacv1.ClusterRoleBinding
	ServiceAccount     *corev1.ServiceAccount
}

func NewRbacGenerator(discovery discovery.DiscoveryInterface, pkg *chartfs.ChartFS, deployName string, deployNamespace string, secretName string, secretNamespace string) *RbacGenerator {
	return &RbacGenerator{
		discovery:       discovery,
		pkg:             pkg,
		deployName:      deployName,
		deployNamespace: deployNamespace,
		secretNamespace: secretNamespace,
		secretName:      secretName,
	}
}

func ptr[T any](x T) *T {
	return &x
}

func containsRule(rules []rbacv1.PolicyRule, rule rbacv1.PolicyRule) bool {
	for _, r := range rules {
		if r.APIGroups[0] == rule.APIGroups[0] && r.Resources[0] == rule.Resources[0] && r.Verbs[0] == rule.Verbs[0] {
			return true
		}
	}
	return false
}
func (r *RbacGenerator) PopulateRBAC(resourceName string) (map[string]RBAC, error) {
	templateDir := path.Join(r.pkg.RootDir(), "templates")
	var rbacErr error
	resources, err := r.getResourcesInfo(templateDir)
	if errors.Is(err, ErrKindApiVersion) {
		rbacErr = err
		err = nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get resources info: %w", err)
	}

	rbacMap := make(map[string]RBAC)

	for _, res := range resources {
		nsName := types.NamespacedName{Name: r.deployName}
		if res.IsNamespaced {
			if res.Namespace == "" && !res.IsNamespaceTemlated {
				res.Namespace = r.deployNamespace
			} else if res.Namespace == "" && res.IsNamespaceTemlated {
				res.IsNamespaced = false
			}
			nsName.Namespace = res.Namespace
		}
		_, ok := rbacMap[nsName.Namespace]
		if !ok {
			rb := RBAC{}
			if res.IsNamespaced {
				// rb.ServiceAccount = ptr(rbactools.CreateServiceAccount(nsName))
				rb.Role = ptr(rbactools.InitRole(resourceName, nsName))
				rb.RoleBinding = ptr(rbactools.CreateRoleBinding(types.NamespacedName{Name: nsName.Name, Namespace: r.deployNamespace}, nsName))
			} else {
				rb.ClusterRole = ptr(rbactools.InitClusterRole(nsName))
				rb.ClusterRole.Rules = append(rb.ClusterRole.Rules,
					rbacv1.PolicyRule{
						APIGroups: []string{""},
						Resources: []string{"namespaces"},
						Verbs:     []string{"get", "list", "watch", "create"},
					})
				rb.ClusterRoleBinding = ptr(rbactools.CreateClusterRoleBinding(types.NamespacedName{Name: r.deployName, Namespace: r.deployNamespace}))
			}
			rbacMap[nsName.Namespace] = rb
		}
		rb := rbacMap[nsName.Namespace]
		apiVSplit := strings.Split(res.APIVersion, "/")
		apiGroup := ""
		if len(apiVSplit) > 1 {
			apiGroup = apiVSplit[0]
		}

		rule := rbacv1.PolicyRule{
			Verbs:     []string{"*"},
			APIGroups: []string{apiGroup},
			Resources: []string{rbactools.KindToResource(res.Kind)},
		}

		if res.IsNamespaced && nsName.Namespace != "" {
			if rb.Role == nil {
				rb.Role = ptr(rbactools.InitRole(resourceName, nsName))
			}
			if !containsRule(rb.Role.Rules, rule) {
				rb.Role.Rules = append(rb.Role.Rules, rule)
			}
		} else {
			if rb.ClusterRole == nil {
				rb.ClusterRole = ptr(rbactools.InitClusterRole(nsName))
			}
			if !containsRule(rb.ClusterRole.Rules, rule) {
				rb.ClusterRole.Rules = append(rb.ClusterRole.Rules, rule)
			}
		}
		rbacMap[nsName.Namespace] = rb
	}
	//Deploy Namespace RBAC
	// Add composition rules in the deploy namespace
	compositionRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"core.krateo.io"},
			Resources: []string{"compositiondefinitions", "compositiondefinitions/status"},
			Verbs:     []string{"*"},
		},
		{
			APIGroups: []string{"composition.krateo.io"},
			Resources: []string{resourceName, fmt.Sprintf("%s/status", resourceName)},
			Verbs:     []string{"*"},
		},
	}

	rb := rbacMap[r.deployNamespace]

	if rb.Role == nil {
		rb.Role = ptr(rbactools.InitRole(resourceName, types.NamespacedName{Name: r.deployName, Namespace: r.deployNamespace}))
	}
	if rb.RoleBinding == nil {
		rb.RoleBinding = ptr(rbactools.CreateRoleBinding(
			types.NamespacedName{Name: r.deployName, Namespace: r.deployNamespace},
			types.NamespacedName{Name: r.deployName, Namespace: r.deployNamespace}))
	}
	if rb.ServiceAccount == nil {
		rb.ServiceAccount = ptr(rbactools.CreateServiceAccount(types.NamespacedName{Name: r.deployName, Namespace: r.deployNamespace}))
	}
	rb.Role.Rules = append(rb.Role.Rules, compositionRules...)
	rbacMap[r.deployNamespace] = rb

	//Secret Namespace RBAC
	if r.secretNamespace != "" && r.secretName != "" {
		rb, ok := rbacMap[r.secretNamespace]
		if !ok {
			rb = RBAC{}
		}
		if rb.Role == nil {
			rb.Role = ptr(rbactools.InitRole(resourceName, types.NamespacedName{Name: r.deployName, Namespace: r.secretNamespace}))
		}
		if rb.RoleBinding == nil {
			rb.RoleBinding = ptr(rbactools.CreateRoleBinding(
				types.NamespacedName{Name: r.deployName, Namespace: r.deployNamespace},
				types.NamespacedName{Name: r.deployName, Namespace: r.secretNamespace}))
		}
		rb.Role.Rules = append(rb.Role.Rules, rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"get"},
			ResourceNames: []string{r.secretName},
		})
		rbacMap[r.secretNamespace] = rb
	}

	if err != nil {
		return nil, err
	}
	return rbacMap, rbacErr
}

func (r *RbacGenerator) getResourcesInfo(templatesDir string) ([]Resource, error) {
	var resources []Resource
	var errs []error
	// error is ignored - not critical
	crdList, _ := crdinfo.GetCRDInfoList(r.pkg)
	dir, err := fs.ReadDir(r.pkg.FS(), templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", templatesDir, err)
	}

	for _, file := range dir {
		if file.IsDir() {
			subdir := path.Join(templatesDir, file.Name())
			ress, err := r.getResourcesInfo(subdir)
			if err != nil {
				return nil, err
			}
			resources = append(resources, ress...)
		}
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			fi, err := r.pkg.Open(path.Join(templatesDir, file.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to open file %s: %w", file.Name(), err)
			}
			scanner := bufio.NewScanner(fi)
			out := new(bytes.Buffer)

			// ^{{\s*-?\s*else\s+if\s+.+}} else if regex
			// ^{{\s*-?\s*if\s+.+}} if regex
			// ^{{\s*-?\s*else\s+}} else regex
			for scanner.Scan() {
				namespaceReg := regexp.MustCompile(`^\s\snamespace:\s*{{(?:[^{}]*|{{.*?}})*}}`)
				finded := namespaceReg.ReplaceAllString(scanner.Text(), "  namespace: krateo-tpl-namespace-auto-generated")
				if finded != scanner.Text() {
					out.WriteString(finded + "\n")
					continue
				}

				ifReg := regexp.MustCompile(`^{{\s*-?\s*if\s+.+}}`)
				finded = ifReg.ReplaceAllString(scanner.Text(), "---")
				if finded != scanner.Text() {
					out.WriteString(finded + "\n")
					continue
				}

				elseIfReg := regexp.MustCompile(`^{{\s*-?\s*else\s+if\s+.+}}`)
				finded = elseIfReg.ReplaceAllString(finded, "---")
				if finded != scanner.Text() {
					out.WriteString(finded + "\n")
					continue
				}

				elseReg := regexp.MustCompile(`^{{\s*-?\s*else\s+}}`)
				finded = elseReg.ReplaceAllString(finded, "---")
				if finded != scanner.Text() {
					out.WriteString(finded + "\n")
					continue
				}

				invalidKindAPIVersionReg := regexp.MustCompile(`{{(?:[^{}]*|{{.*?}})*}}`)
				finded = invalidKindAPIVersionReg.ReplaceAllString(scanner.Text(), "")
				out.WriteString(finded + "\n")
			}

			dividerReg := regexp.MustCompile(`^-{3}$`)
			yamls := dividerReg.Split(out.String(), -1)

			for _, y := range yamls {
				n, _ := yaml.Parse(y)
				if n.IsNilOrEmpty() {
					continue
				}

				res := Resource{
					Kind:       n.GetKind(),
					APIVersion: n.GetApiVersion(),
				}
				ns := n.GetNamespace()
				if ns == "krateo-tpl-namespace-auto-generated" {
					ns = ""
					res.IsNamespaceTemlated = true
				}
				res.Namespace = ns
				if res.Kind == "" || res.APIVersion == "" {
					errs = append(errs, fmt.Errorf("%s: %s %s %s", file.Name(), res.Kind, res.APIVersion, res.Namespace))
					continue
				}
				apiVSplit := strings.Split(n.GetApiVersion(), "/")
				gr := schema.GroupResource{
					Resource: strings.ToLower(flect.Pluralize(n.GetKind())),
				}
				if len(apiVSplit) > 1 {
					gr.Group = apiVSplit[0]
				}

				res.IsNamespaced, err = isGRNamespaced(r.discovery, gr, crdList)
				if err != nil {
					//Default to clusterRole
					res.IsNamespaced = false
				}

				resources = append(resources, res)
			}
		}
	}

	return resources, wrapErrors(errs)
}

func wrapErrors(errs []error) error {
	var wrappedErrs error
	if len(errs) == 0 {
		return nil
	}
	for _, err := range errs {
		wrappedErrs = fmt.Errorf("%s - ", err)
	}
	return fmt.Errorf("%w: %s", ErrKindApiVersion, wrappedErrs)
}

func isGRNamespaced(discovery discovery.DiscoveryInterface, gr schema.GroupResource, crdList []crdinfo.CRDInfo) (bool, error) {
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
