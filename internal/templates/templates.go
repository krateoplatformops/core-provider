package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"text/template"
)

var (
	//go:embed assets/sa.yaml
	saTpl string

	//go:embed assets/role.yaml
	roleTpl string

	//go:embed assets/binding.yaml
	bindingTpl string

	//go:embed assets/deployment.yaml
	deploymentTpl string
)

type TemplateType string

const (
	ServiceAccount TemplateType = "sa"
	Role           TemplateType = "role"
	RoleBinding    TemplateType = "binding"
	Deployment     TemplateType = "deployment"
)

type Renderoptions struct {
	Group     string
	Version   string
	Resource  string
	Namespace string
	Name      string
	Tag       string
}

func Values(opts Renderoptions) map[string]string {
	if len(opts.Name) == 0 {
		opts.Name = fmt.Sprintf("%s-controller", opts.Resource)
	}

	if len(opts.Namespace) == 0 {
		opts.Namespace = "default"
	}

	if len(opts.Tag) == 0 {
		opts.Tag = os.Getenv("COMPOSITION_CONTROLLER_IMAGE_TAG")
	}

	return map[string]string{
		"apiGroup":   opts.Group,
		"apiVersion": opts.Version,
		"resource":   opts.Resource,
		"name":       opts.Name,
		"namespace":  opts.Namespace,
		"tag":        opts.Tag,
	}
}

func Render(tt TemplateType, values map[string]string) ([]byte, error) {
	switch tt {
	case ServiceAccount:
		return execute(string(tt), saTpl, values)
	case Role:
		return execute(string(tt), roleTpl, values)
	case RoleBinding:
		return execute(string(tt), bindingTpl, values)
	case Deployment:
		return execute(string(tt), deploymentTpl, values)
	default:
		return nil, fmt.Errorf("unable to find template for type: %s", string(tt))
	}
}

func execute(name string, content string, data map[string]string) ([]byte, error) {
	tpl, err := template.New(name).Funcs(TxtFuncMap()).Parse(content)
	if err != nil {
		return nil, err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
