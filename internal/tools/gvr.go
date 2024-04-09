package tools

import (
	"fmt"
	"io"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/internal/strutil"
	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
)

func GroupVersionKind(fs *chartfs.ChartFS) (schema.GroupVersionKind, error) {
	fin, err := fs.Open(fs.RootDir() + "/Chart.yaml")
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	defer fin.Close()

	din, err := io.ReadAll(fin)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	res := map[string]any{}
	if err := yaml.Unmarshal(din, &res); err != nil {
		return schema.GroupVersionKind{}, err
	}

	return schema.GroupVersionKind{
		Group:   "composition.krateo.io",
		Version: fmt.Sprintf("v%s", strings.ReplaceAll(res["version"].(string), ".", "-")),
		Kind:    flect.Pascalize(strutil.ToGolangName(res["name"].(string))),
	}, nil
}

func ToGroupVersionResource(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:   gvk.Group,
		Version: gvk.Version,
		Resource: namer.NewPrivatePluralNamer(nil).Name(
			&types.Type{Name: types.Name{Name: gvk.Kind}},
		),
	}
}
