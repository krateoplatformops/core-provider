package chartfs

import (
	"fmt"
	"io"
	"strings"

	"github.com/gobuffalo/flect"

	"github.com/krateoplatformops/core-provider/internal/tools/strutil"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GroupVersionKind(fs *ChartFS) (schema.GroupVersionKind, error) {
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
