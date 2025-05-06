package defaults

import (
	"encoding/json"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// PopulateDefaultsFromCRD applies default values from the CRD's OpenAPI schema to the custom resource spec.
// It uses the OpenAPI schema to find default values for fields in the spec and sets them in the custom resource (only if they are not already set).
func PopulateDefaultsFromCRD(crd *apiextensionsv1.CustomResourceDefinition, cr *runtime.Object) error {
	// Validate inputs
	if crd == nil {
		return fmt.Errorf("CustomResourceDefinition is nil")
	}
	if cr == nil || *cr == nil {
		return fmt.Errorf("custom resource is nil")
	}

	// Find the appropriate version schema
	// First, try to get the schema from the stored version
	var schema *apiextensionsv1.JSONSchemaProps
	crVersion := ""

	// Extract version from the custom resource
	crUnstructured, ok := (*cr).(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("custom resource must be an *unstructured.Unstructured")
	}

	crAPIVersion := crUnstructured.GetAPIVersion()
	parts := strings.Split(crAPIVersion, "/")
	if len(parts) == 2 {
		crVersion = parts[1]
	} else {
		return fmt.Errorf("invalid apiVersion format in custom resource: %s", crAPIVersion)
	}

	// Find the schema for the resource version
	for _, version := range crd.Spec.Versions {
		if version.Name == crVersion && version.Schema != nil && version.Schema.OpenAPIV3Schema != nil {
			schema = version.Schema.OpenAPIV3Schema
			break
		}
	}

	if schema == nil {
		return fmt.Errorf("no schema found for version %s in CRD %s", crVersion, crd.Name)
	}

	// Now find the spec field in the schema
	specSchema, found := schema.Properties["spec"]
	if !found {
		// No spec field defined in the schema
		return nil
	}

	// Get the current spec from the custom resource
	spec, found, err := unstructured.NestedFieldCopy(crUnstructured.Object, "spec")
	if err != nil {
		return fmt.Errorf("error accessing spec field: %w", err)
	}

	if !found || spec == nil {
		// No spec field in the CR, initialize it
		spec = map[string]interface{}{}
	}

	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return fmt.Errorf("spec field is not a map")
	}

	// Recursively apply defaults
	err = applyDefaultsToMap(specSchema.Properties, specMap)
	if err != nil {
		return err
	}

	// Update the spec in the CR
	err = unstructured.SetNestedField(crUnstructured.Object, specMap, "spec")
	if err != nil {
		return fmt.Errorf("error setting updated spec: %w", err)
	}

	return nil
}

// applyDefaultsToMap recursively applies default values to a map according to the schema
func applyDefaultsToMap(properties map[string]apiextensionsv1.JSONSchemaProps, targetMap map[string]interface{}) error {
	for fieldName, fieldSchema := range properties {
		// Skip if the field is already set
		if _, exists := targetMap[fieldName]; exists {
			// If it's an object, recurse into it
			if fieldSchema.Type == "object" && len(fieldSchema.Properties) > 0 {
				fieldValue, ok := targetMap[fieldName].(map[string]interface{})
				if !ok {
					// Field exists but is not a map, skip recursion
					continue
				}

				// Recursively apply defaults to the nested object
				if err := applyDefaultsToMap(fieldSchema.Properties, fieldValue); err != nil {
					return err
				}
			}
			continue
		}

		// If there's a default value, apply it
		if fieldSchema.Default != nil {
			var defaultValue interface{}
			if err := json.Unmarshal(fieldSchema.Default.Raw, &defaultValue); err != nil {
				return fmt.Errorf("failed to unmarshal default value for field %s: %w", fieldName, err)
			}

			// Convert float64 to int64 if needed based on the field type
			if fieldSchema.Type == "integer" {
				if floatVal, ok := defaultValue.(float64); ok {
					// Convert float64 to int64 for integer fields
					defaultValue = int64(floatVal)
				}
			}

			targetMap[fieldName] = defaultValue
		} else if fieldSchema.Type == "object" && len(fieldSchema.Properties) > 0 {
			// Create nested object and apply defaults recursively
			nestedMap := map[string]interface{}{}
			if err := applyDefaultsToMap(fieldSchema.Properties, nestedMap); err != nil {
				return err
			}

			// Only add the nested map if it has any values
			if len(nestedMap) > 0 {
				targetMap[fieldName] = nestedMap
			}
		} else if fieldSchema.Type == "array" && fieldSchema.Items != nil && fieldSchema.Items.Schema != nil {
			// Handle array defaults if needed
			if fieldSchema.MinItems != nil && *fieldSchema.MinItems > 0 && fieldSchema.Items.Schema.Default != nil {
				var defaultItem interface{}
				if err := json.Unmarshal(fieldSchema.Items.Schema.Default.Raw, &defaultItem); err != nil {
					return fmt.Errorf("failed to unmarshal default array item for field %s: %w", fieldName, err)
				}

				// Convert float64 to int64 if needed for array items
				if fieldSchema.Items.Schema.Type == "integer" {
					if floatVal, ok := defaultItem.(float64); ok {
						defaultItem = int64(floatVal)
					}
				}

				defaultArray := make([]interface{}, *fieldSchema.MinItems)
				for i := range defaultArray {
					defaultArray[i] = defaultItem
				}

				targetMap[fieldName] = defaultArray
			}
		}
	}

	return nil
}
