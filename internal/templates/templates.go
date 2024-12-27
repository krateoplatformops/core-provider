package templates

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

var (
	//go:embed assets/deployment.yaml
	deploymentTpl string
)

type Renderoptions struct {
	Group     string
	Version   string
	Resource  string
	Namespace string
	Name      string
	Tag       string
	Env       map[string]string
}

func Values(opts Renderoptions) map[string]any {
	if len(opts.Name) == 0 {
		opts.Name = fmt.Sprintf("%s-controller", opts.Resource)
	}

	if len(opts.Namespace) == 0 {
		opts.Namespace = "default"
	}

	values := map[string]any{
		"apiGroup":   opts.Group,
		"apiVersion": opts.Version,
		"resource":   opts.Resource,
		"name":       opts.Name,
		"namespace":  opts.Namespace,
		"tag":        opts.Tag,
	}

	if len(opts.Env) > 0 {
		values["extraEnv"] = map[string]string{}
		for k, v := range opts.Env {
			values["extraEnv"].(map[string]string)[k] = v
		}
	}

	return values
}

func RenderDeployment(values map[string]any) ([]byte, error) {
	tpl, err := template.New("deployment").Funcs(TxtFuncMap()).Parse(deploymentTpl)
	if err != nil {
		return nil, err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, values); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
