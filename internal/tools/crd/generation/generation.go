package generation

import (
	"fmt"

	_ "embed"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	hasher "github.com/krateoplatformops/core-provider/internal/tools/hash"
	"github.com/krateoplatformops/crdgen/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//go:embed statics/status.schema.json
var statusJsonSchema []byte

//go:embed statics/empty.schema.json
var emptyJsonSchema []byte

func UpdateCABundle(crd *apiextensionsv1.CustomResourceDefinition, caBundle []byte) error {
	if crd.Spec.Conversion == nil {
		return fmt.Errorf(".spec.conversion field is nil")
	}
	if crd.Spec.Conversion.Webhook == nil {
		return fmt.Errorf(".spec.conversion.webhook field is nil")
	}
	if crd.Spec.Conversion.Webhook.ClientConfig == nil {
		return fmt.Errorf(".spec.conversion.webhook.clientConfig field is nil")
	}

	crd.Spec.Conversion.Webhook.ClientConfig.CABundle = caBundle
	return nil
}

func SetServedStorage(crd *apiextensionsv1.CustomResourceDefinition, version string, served, storage bool) {
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name == version {
			crd.Spec.Versions[i].Served = served
			crd.Spec.Versions[i].Storage = storage
		}
	}
}

// AppendVersion appends the version of the toadd CRD to the crd CRD and sets the Storage and Served fields in the last version of the crd CRD.
func AppendVersion(crd apiextensionsv1.CustomResourceDefinition, toadd apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
	for _, el2 := range toadd.Spec.Versions {
		exist := false
		vacuum := false
		for _, el1 := range crd.Spec.Versions {
			if el1.Name == el2.Name {
				exist = true
				break
			}
		}
		for _, el1 := range crd.Spec.Versions {
			if el1.Name == "vacuum" {
				vacuum = true
				break
			}
		}

		if !exist {
			crd.Spec.Versions = append(crd.Spec.Versions, el2)
			if !vacuum {
				crd.Spec.Versions = append(crd.Spec.Versions, apiextensionsv1.CustomResourceDefinitionVersion{
					Name:    "vacuum",
					Served:  false,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:        "object",
							Description: "This is a vacuum version to storage different versions",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"apiVersion": {
									Type:        "string",
									Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
								},
								"kind": {
									Type: "string",
								},
								"metadata": {
									Type: "object",
								},
								"spec": {
									Type:                   "object",
									XPreserveUnknownFields: &[]bool{true}[0],
								},
								"status": {
									Type:                   "object",
									XPreserveUnknownFields: &[]bool{true}[0],
								},
							},
						},
					},
				})
			}
			for i := range crd.Spec.Versions {
				// if different from vacuum served: false and storage: true
				if crd.Spec.Versions[i].Name != "vacuum" {
					crd.Spec.Versions[i].Served = true
					crd.Spec.Versions[i].Storage = false
				}
			}
		}
	}

	return &crd, nil
}

func UpdateStatus(crd *apiextensionsv1.CustomResourceDefinition, version apiextensionsv1.CustomResourceDefinitionVersion) error {
	if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
		return fmt.Errorf("CRD %s version %s schema is nil", crd.Name, version.Name)
	}
	newStatus := version.Schema.OpenAPIV3Schema.Properties["status"]

	// Update the status schema for all versions
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Schema != nil && crd.Spec.Versions[i].Schema.OpenAPIV3Schema != nil {
			crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"] = newStatus
		}
	}
	return nil
}
func Unmarshal(dat []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &apiextensionsv1.CustomResourceDefinition{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

func GenerateCRD(specSchema []byte, gvk schema.GroupVersionKind) (*apiextensionsv1.CustomResourceDefinition, error) {
	bcrd, err := generateCRD(specSchema, gvk, false)
	if err != nil {
		return nil, fmt.Errorf("error generating CRD: %w", err)
	}
	crd, err := Unmarshal(bcrd)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling generated CRD: %w", err)
	}
	return crd, nil
}

func StatusEqual(crd1, crd2 *apiextensionsv1.CustomResourceDefinition) (bool, error) {
	crdHasher := hasher.NewFNVObjectHash()
	i1 := 0

	searchFirstStatus := func(crd *apiextensionsv1.CustomResourceDefinition) int {
		for i, v := range crd.Spec.Versions {
			if v.Schema != nil && v.Schema.OpenAPIV3Schema != nil {
				if _, ok := v.Schema.OpenAPIV3Schema.Properties["status"]; ok {
					return i
				}
			}
		}
		return -1
	}
	i1 = searchFirstStatus(crd1)
	if i1 == -1 {
		return false, fmt.Errorf("CRD %s has no version with status property", crd1.Name)
	}
	err := crdHasher.SumHash(crd1.Spec.Versions[i1].Schema.OpenAPIV3Schema.Properties["status"])
	if err != nil {
		return false, fmt.Errorf("error hashing CRD status: %w", err)
	}
	genCRDHasher := hasher.NewFNVObjectHash()

	i2 := searchFirstStatus(crd2)
	if i2 == -1 {
		return false, fmt.Errorf("CRD %s has no version with status property", crd2.Name)
	}
	err = genCRDHasher.SumHash(crd2.Spec.Versions[i2].Schema.OpenAPIV3Schema.Properties["status"])
	if err != nil {
		return false, fmt.Errorf("error hashing generated CRD status: %w", err)
	}

	return crdHasher.GetHash() == genCRDHasher.GetHash(), nil
}

func GetGVRFromGeneratedCRD(specSchema []byte, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	bcrd, err := generateCRD(specSchema, gvk, true)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("error generating CRD for GVR fallback: %w", err)
	}
	crd, err := Unmarshal(bcrd)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("error unmarshalling generated CRD for GVR fallback: %w", err)
	}
	gvr := schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  crd.Spec.Versions[0].Name,
		Resource: crd.Spec.Names.Plural,
	}
	return gvr, nil
}

func generateCRD(specSchema []byte, gvk schema.GroupVersionKind, onlyMetadata bool) ([]byte, error) {
	var statusSchema []byte
	var err error

	// Read static status schema
	statusSchema = statusJsonSchema

	if onlyMetadata {
		specSchema = []byte(emptyJsonSchema)
		statusSchema = []byte(emptyJsonSchema)
	}

	res, err := crdgen.Generate(crdgen.Options{
		Group:        gvk.Group,
		Version:      gvk.Version,
		Kind:         gvk.Kind,
		Managed:      true,
		Categories:   []string{"compositions", "comps"},
		SpecSchema:   specSchema,
		StatusSchema: statusSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("generating crd: %w", err)
	}
	return res, nil
}
