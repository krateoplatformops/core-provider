package compositiondefinitions

import (
	"context"
	"fmt"
	"io"
	"strings"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/crdgen"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
)

const (
	statusExtra = `
{
  "$schema": "http://json-schema.org",
  "type": "object",
  "properties": {
    "helmChartUrl": {
	  "optional": true,
      "type": "string"
    },
    "helmChartVersion": {
	  "optional": true,
      "type": "string"
    },
    "managed": {
	  "optional": true,
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "apiVersion": {
            "type": "string"
          },
          "resource": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "namespace": {
            "type": "string"
          }
        }
      }
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

func resourceNamer(resourceName string, chartVersion string) string {
	return fmt.Sprintf("%s-%s-controller", resourceName, chartVersion)
}

// updateVersionInfo updates the version information of a CompositionDefinition custom resource
// based on the provided CustomResourceDefinition and GroupVersionResource.
//
// The function iterates through the versions specified in the CustomResourceDefinition and updates
// the corresponding version information in the CompositionDefinition's status. If a version is not
// found in the existing status, it is added. If the version matches the GroupVersionResource, additional
// chart information is populated from the CompositionDefinition's spec.
func updateVersionInfo(cr *compositiondefinitionsv1alpha1.CompositionDefinition, crd *apiextensionsv1.CustomResourceDefinition, gvr schema.GroupVersionResource) {
	for _, v := range crd.Spec.Versions {
		i := -1
		for j, cv := range cr.Status.Managed.VersionInfo {
			if cv.Version == v.Name {
				i = j
				break
			}
		}

		if i == -1 {
			var versionDetail compositiondefinitionsv1alpha1.VersionDetail
			versionDetail.Version = v.Name
			versionDetail.Served = v.Served
			versionDetail.Stored = v.Storage

			if gvr.Version == versionDetail.Version {
				versionDetail.Chart = &compositiondefinitionsv1alpha1.ChartInfoProps{}
				versionDetail.Chart.Credentials = cr.Spec.Chart.Credentials
				versionDetail.Chart.InsecureSkipVerifyTLS = cr.Spec.Chart.InsecureSkipVerifyTLS
				versionDetail.Chart.Repo = cr.Spec.Chart.Repo
				versionDetail.Chart.Url = cr.Spec.Chart.Url
				versionDetail.Chart.Version = cr.Spec.Chart.Version
			}

			cr.Status.Managed.VersionInfo = append(cr.Status.Managed.VersionInfo, versionDetail)
			continue
		}
		cr.Status.Managed.VersionInfo[i].Served = v.Served
		cr.Status.Managed.VersionInfo[i].Stored = v.Storage
	}
}

type CompositionsInfo struct {
	GVR       schema.GroupVersionResource
	Namespace string
}

const (
	CompositionVersionLabel = "krateo.io/composition-version"
)

// updateCompositionsVersion updates the version label of all compositions in a namespace
// that match the specified GroupVersionResource (GVR) and current version.
func updateCompositionsVersion(ctx context.Context, dyn dynamic.Interface, log logging.Logger, opts CompositionsInfo, newVersion string) error {
	// Create a label requirement for the composition version
	labelreq, err := labels.NewRequirement(CompositionVersionLabel, selection.Equals, []string{opts.GVR.Version})
	if err != nil {
		return err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*labelreq)

	ul, err := dyn.Resource(opts.GVR).Namespace(opts.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return fmt.Errorf("error listing compositions: %s", err)
	}

	if len(ul.Items) == 0 {
		log.Info("No compositions found", "Group", opts.GVR.Group, "Version", opts.GVR.Version, "Resource", opts.GVR.Resource)
	}

	for _, u := range ul.Items {
		labelmap, ok, err := unstructured.NestedStringMap(u.Object, "metadata", "labels")
		if err != nil {
			return err
		}
		if !ok {
			labelmap = make(map[string]string)
		}

		labelmap[CompositionVersionLabel] = newVersion
		err = unstructured.SetNestedStringMap(u.Object, labelmap, "metadata", "labels")
		if err != nil {
			return err
		}

		_, err = dyn.Resource(opts.GVR).Namespace(opts.Namespace).Update(ctx, &u, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating compositions: %s", err)
		}
	}

	return nil
}
