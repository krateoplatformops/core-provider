package compositiondefinitions

import (
	"io"
	"strings"

	"github.com/krateoplatformops/crdgen"
)

const (
	statusExtra = `
{
  "$schema": "http://json-schema.org",
  "type": "object",
  "properties": {
    "helmChartUrl": {
      "type": "string"
    },
    "helmChartVersion": {
      "type": "string"
    }
  }
}`
)

var _ crdgen.JsonSchemaGetter = (*staticJsonSchemaGetter)(nil)

func StaticJsonSchemaGetter() crdgen.JsonSchemaGetter {
	return &staticJsonSchemaGetter{}
}

type staticJsonSchemaGetter struct {
}

func (f *staticJsonSchemaGetter) Get() ([]byte, error) {
	return io.ReadAll(strings.NewReader(statusExtra))
}
