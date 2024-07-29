package rbacgen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	crdinfo "github.com/krateoplatformops/core-provider/internal/tools/crdInfo"
	"github.com/krateoplatformops/core-provider/internal/tools/rbactools"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const strErrKindApiVersion = "failed to find kind and apiVersion"

var ErrKindApiVersion = errors.New(strErrKindApiVersion)

type Resource struct {
	Kind                string
	Resource            string
	APIVersion          string
	Namespace           string
	IsNamespaceTemlated bool
	IsNamespaced        bool
}

type RbacGenerator struct {
	discovery       discovery.CachedDiscoveryInterface
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

func NewRbacGenerator(discovery discovery.CachedDiscoveryInterface, pkg *chartfs.ChartFS, deployName string, deployNamespace string, secretName string, secretNamespace string) *RbacGenerator {
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
			Resources: []string{res.Resource},
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
	// Deploy Namespace RBAC
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
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
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

func getDependencyDir(pkg *chartfs.ChartFS, templatesDir string) string {
	chartsDirPath := path.Join(strings.TrimSuffix(templatesDir, "templates"), "charts")

	_, err := fs.ReadDir(pkg.FS(), chartsDirPath)
	if err != nil {
		return ""
	}
	return chartsDirPath
}

func (r *RbacGenerator) getResourcesInfo(templatesDir string) ([]Resource, error) {
	var resources []Resource
	var errs []error
	// error is ignored - not critical
	crdList, _ := crdinfo.GetCRDInfoList(r.pkg)

	depsDirPath := getDependencyDir(r.pkg, templatesDir)
	if depsDirPath != "" {
		depsDir, err := fs.ReadDir(r.pkg.FS(), depsDirPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", depsDirPath, err)
		}
		for _, file := range depsDir {
			if file.IsDir() {
				depsDir := path.Join(depsDirPath, file.Name(), "templates")
				ress, err := r.getResourcesInfo(depsDir)
				if err != nil {
					return nil, err
				}
				resources = append(resources, ress...)
			}
		}
	}

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

				cleanTemplatedFields := regexp.MustCompile(`:\s*{{(?:[^{}]*|{{.*?}})*}}`)
				finded = cleanTemplatedFields.FindString(scanner.Text())
				if finded != "" {
					cleanTemplatedFieldReg := regexp.MustCompile(`:\s*(.*)`)
					finded = cleanTemplatedFieldReg.ReplaceAllString(finded, "")
					if finded != scanner.Text() {
						out.WriteString(finded + "\n")
						continue
					}
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

				dots := regexp.MustCompile(`^\.\.\.\s*$`)
				finded = dots.ReplaceAllString(scanner.Text(), "")
				if dots.MatchString(scanner.Text()) {
					out.WriteString(finded + "\n")
					continue
				}

				out.WriteString(finded + "\n")
			}

			strout := out.String()
			out.Reset()
			templateReg := regexp.MustCompile(`{{(?:[^{}]*|{{.*?}})*}}`)
			outBuf := templateReg.ReplaceAllString(strout, "")
			out.WriteString(outBuf + "\n")

			// now scan the output out buffer
			var yamls []string
			scanner = bufio.NewScanner(out)
			tmpout := new(bytes.Buffer)
			for scanner.Scan() {
				// regex to match the divider
				dividerReg := regexp.MustCompile(`^-{3}\s*$`)
				// if the line is a divider, then we have a yaml so put it in the yamls slice
				if dividerReg.MatchString(scanner.Text()) {
					yamls = append(yamls, tmpout.String())
					tmpout.Reset()
					continue
				}
				tmpout.WriteString(scanner.Text() + "\n")
			}
			yamls = append(yamls, tmpout.String())

			for _, y := range yamls {
				n, err := yaml.Parse(y)
				if err != nil && err != io.EOF {
					errs = append(errs, fmt.Errorf("failed to parse yaml in file %s : %w", file.Name(), err))
					continue
				}
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

				gk := schema.GroupKind{
					Kind: n.GetKind(),
				}
				if len(apiVSplit) > 1 {
					gk.Group = apiVSplit[0]
				}

				res.Resource, res.IsNamespaced, err = getGKInfo(r.discovery, gk, crdList)
				if err != nil {
					//Default to clusterRole
					res.IsNamespaced = false
					errs = append(errs, fmt.Errorf("failed to get resource info for %s %s: %w", res.Kind, res.APIVersion, err))
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

// returns true if the resource is namespaced or false if it's cluster scoped
func boolForScope(scope meta.RESTScope) bool {
	return scope == meta.RESTScopeNamespace
}

func getGKInfo(discovery discovery.CachedDiscoveryInterface, gk schema.GroupKind, crdList []crdinfo.CRDInfo) (string, bool, error) {
	rm := restmapper.NewDeferredDiscoveryRESTMapper(discovery)

	for _, crd := range crdList {
		if crd.GroupVersion.Group == gk.Group && crd.Kind == gk.Kind {
			return crd.Resource, crd.Namespaced, nil
		}
	}

	gvr, err := rm.RESTMapping(gk)
	if err != nil {
		return "", false, err
	}

	return gvr.Resource.Resource, boolForScope(gvr.Scope), nil
}
