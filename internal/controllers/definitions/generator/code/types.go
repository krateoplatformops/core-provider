package code

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/text"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/transpiler"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/transpiler/jsonschema"
)

const (
	pkgCommon           = "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	pkgCommonAlias      = "rtv1"
	pkgMeta             = "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgMetaAlias        = "metav1"
	pkgSpecCommentFmt   = "%s defines the desired state of %s"
	pkgStatusCommentFmt = "%s defines the observed state of %s"
)

func CreateTypesDotGo(workdir string, res *Resource) error {
	srcdir, err := createSourceDir(workdir, res)
	if err != nil {
		return err
	}

	info, err := jsonschemaToStruct(bytes.NewReader(res.Schema))
	if err != nil {
		return err
	}

	kind := text.ToGolangName(res.Kind)

	g := jen.NewFile(res.Version)
	g.ImportAlias(pkgCommon, pkgCommonAlias)
	g.ImportAlias(pkgMeta, pkgMetaAlias)

	for k, v := range info {
		g.Add(renderStruct(k, v, res))
	}

	g.Add(jen.Line())
	g.Add(createFailedObjectRef())
	g.Add(jen.Line())
	g.Add(createStatusStruct(res.Kind))

	g.Add(jen.Comment("+kubebuilder:object:root=true"))
	g.Add(jen.Comment("+kubebuilder:subresource:status"))
	if len(res.Categories) > 0 {
		g.Add(jen.Comment(
			fmt.Sprintf("+kubebuilder:resource:scope=Namespaced,categories={%s}",
				strings.Join(res.Categories, ","))))
	} else {
		g.Add(jen.Comment("+kubebuilder:resource:scope=Namespaced"))
	}
	g.Add(jen.Comment(`+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"`).Line())
	g.Add(jen.Line())

	g.Add(jen.Type().Id(kind).Struct(
		jen.Qual(pkgMeta, "TypeMeta").Tag(map[string]string{"json": ",inline"}),
		jen.Qual(pkgMeta, "ObjectMeta").Tag(map[string]string{"json": ",inline"}),
		jen.Line(),
		jen.Id("Spec").Id(fmt.Sprintf("%sSpec", kind)).Tag(map[string]string{"json": "spec,omitempty"}),
		jen.Id("Status").Id(fmt.Sprintf("%sStatus", kind)).Tag(map[string]string{"json": "status,omitempty"}),
	).Line())

	g.Add(jen.Comment("+kubebuilder:object:root=true"))
	g.Add(jen.Line())

	g.Add(jen.Type().Id(fmt.Sprintf("%sList", kind)).Struct(
		jen.Qual(pkgMeta, "TypeMeta").Tag(map[string]string{"json": ",inline"}),
		jen.Qual(pkgMeta, "ListMeta").Tag(map[string]string{"json": "metadata,omitempty"}),
		jen.Line(),
		jen.Id("Items").Id(fmt.Sprintf("[]%s", kind)).Tag(map[string]string{"json": "items"}),
	).Line())

	src, err := os.Create(filepath.Join(srcdir, "types.go"))
	if err != nil {
		return err
	}
	defer src.Close()

	return g.Render(src)
}

func renderStruct(key string, el transpiler.Struct, res *Resource) jen.Code {
	kind := text.ToGolangName(res.Kind)

	fields := []jen.Code{}

	root := key == "Root"
	if root {
		key = text.ToGolangName(fmt.Sprintf("%sSpec", kind))
		fields = append(fields,
			jen.Qual(pkgCommon, "ManagedSpec").
				Tag(map[string]string{
					"json": ",inline",
				}).Line())
	}

	for _, f := range el.Fields {
		fields = append(fields, renderField(f))
	}

	if root {
		comment := fmt.Sprintf(pkgSpecCommentFmt, key, kind)
		return jen.Comment(comment).Line().
			Type().Id(key).Struct(fields...).
			Line()
	}

	return jen.Type().Id(key).Struct(fields...).Line()
}

func renderField(el transpiler.Field) jen.Code {
	res := &jen.Statement{}
	if len(el.Description) > 0 {
		comment := fmt.Sprintf("%s: %s", el.Name, el.Description)
		res.Add(jen.Comment(comment).Line())
	}

	if el.Optional {
		res.Add(jen.Comment("+optional").Line())
		res.Add(jen.Id(el.Name).Op("*").Id(el.Type))
	} else {
		res.Add(jen.Id(el.Name).Id(el.Type))
	}

	res.Add(jen.Tag(map[string]string{
		"json": fmt.Sprintf("%s,omitempty", el.JSONName),
	}).Line())

	return res
}

func createStatusStruct(kind string) jen.Code {
	kind = text.ToGolangName(kind)
	key := text.ToGolangName(fmt.Sprintf("%sStatus", kind))

	fields := []jen.Code{
		jen.Qual(pkgCommon, "ManagedStatus").Tag(map[string]string{
			"json": ",inline",
		}),
		jen.Id("FailedObjectRef").Op("*").Id("FailedObjectRef").Tag(map[string]string{
			"json": "failedObjectRef,omitempty",
		}),
		jen.Id("HelmChartUrl").Op("*").Id("string").Tag(map[string]string{
			"json": "helmChartUrl,omitempty",
		}),
		jen.Id("HelmChartVersion").Op("*").Id("string").Tag(map[string]string{
			"json": "helmChartVersion,omitempty",
		}),
	}

	comment := fmt.Sprintf(pkgStatusCommentFmt, key, kind)
	return jen.Comment(comment).Line().
		Type().Id(key).Struct(fields...).
		Line()
}

/*
func createStatusStruct(res *Resource) jen.Code {
	kind := text.ToGolangName(res.Kind)
	key := text.ToGolangName(fmt.Sprintf("%sStatus", kind))

	field1 := jen.Qual(pkgCommon, "ManagedStatus").
		Tag(map[string]string{
			"json": ",inline",
		}).Line()

	comment := fmt.Sprintf(pkgStatusCommentFmt, key, kind)
	return jen.Comment(comment).Line().
		Type().Id(key).Struct(field1).
		Line()
}
*/

func createFailedObjectRef() jen.Code {
	meta := []transpiler.Field{
		{
			Name:        "APIVersion",
			JSONName:    "apiVersion",
			Description: "API version of the object.",
			Optional:    false,
			Type:        "string",
		},
		{
			Name:        "Kind",
			JSONName:    "kind",
			Description: "Kind of the object.",
			Optional:    false,
			Type:        "string",
		},
		{
			Name:        "Name",
			JSONName:    "name",
			Description: "Name of the object.",
			Optional:    false,
			Type:        "string",
		},
		{
			Name:        "Namespace",
			JSONName:    "namespace",
			Description: "Namespace of the object.",
			Optional:    false,
			Type:        "string",
		},
	}

	fields := []jen.Code{}
	for _, el := range meta {
		fields = append(fields, renderField(el))
	}

	return jen.Type().Id("FailedObjectRef").Struct(fields...).Line()
}

func createSourceDir(workdir string, res *Resource) (string, error) {
	srcdir := filepath.Join(workdir, "apis", strings.ToLower(res.Kind), res.Version)
	err := os.MkdirAll(srcdir, os.ModePerm)
	return srcdir, err
}

func jsonschemaToStruct(r io.Reader) (map[string]transpiler.Struct, error) {
	schema, err := jsonschema.ParseReader(r)
	if err != nil {
		return nil, err
	}

	return transpiler.Transpile(schema)
}
