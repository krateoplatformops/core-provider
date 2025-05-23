package compositiondefinitions

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/krateoplatformops/core-provider/internal/tools/kube"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/core-provider/internal/tools/objects"
	"github.com/krateoplatformops/crdgen"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
    },
    "managed": {
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
	return fmt.Sprintf("%s-%s", resourceName, chartVersion)
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

func getCompositions(ctx context.Context, dyn dynamic.Interface, log func(msg string, keysAndValues ...any), gvr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	// Create a label requirement for the composition version
	labelreq, err := labels.NewRequirement(deploy.CompositionVersionLabel, selection.Equals, []string{gvr.Version})
	if err != nil {
		log("Error creating label requirement", "error", err)
		return nil, fmt.Errorf("error creating label requirement: %w", err)
	}
	selector := labels.NewSelector()
	selector = selector.Add(*labelreq)

	ul, err := dyn.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		log("Error listing compositions", "error", err)
		return nil, fmt.Errorf("error listing compositions: %w", err)
	}

	return ul, nil
}

// updateCompositionsVersion updates the version label of all compositions in a namespace
// that match the specified GroupVersionResource (GVR) and current version.
func updateCompositionsVersion(ctx context.Context, dyn dynamic.Interface, log func(msg string, keysAndValues ...any), gvr schema.GroupVersionResource, newVersion string) error {
	ul, err := getCompositions(ctx, dyn, log, gvr)
	if err != nil {
		return fmt.Errorf("error getting compositions: %w", err)
	}

	if len(ul.Items) == 0 {
		log("No compositions found for the specified GVR and version")
		return nil
	}

	for _, u := range ul.Items {
		labelmap, ok, err := unstructured.NestedStringMap(u.Object, "metadata", "labels")
		if err != nil {
			return err
		}
		if !ok {
			labelmap = make(map[string]string)
		}

		labelmap[deploy.CompositionVersionLabel] = newVersion
		err = unstructured.SetNestedStringMap(u.Object, labelmap, "metadata", "labels")
		if err != nil {
			return err
		}

		_, err = dyn.Resource(gvr).Namespace(u.GetNamespace()).Update(ctx, &u, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating compositions: %s", err)
		}
	}

	return nil
}

func getCompositionDefinitions(ctx context.Context, cli client.Client, gk schema.GroupKind) ([]compositiondefinitionsv1alpha1.CompositionDefinition, error) {
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err := cli.List(ctx, &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return nil, fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}

	lst := []compositiondefinitionsv1alpha1.CompositionDefinition{}
	for i := range cdList.Items {
		cd := &cdList.Items[i]
		if cd.Status.Managed.Group == gk.Group &&
			cd.Status.Managed.Kind == gk.Kind {
			lst = append(lst, *cd)
		}
	}

	return lst, nil
}
func getCompositionDefinitionsWithVersion(ctx context.Context, cli client.Client, gk schema.GroupKind, chartVersion string) ([]compositiondefinitionsv1alpha1.CompositionDefinition, error) {
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err := cli.List(ctx, &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return nil, fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}

	lst := []compositiondefinitionsv1alpha1.CompositionDefinition{}
	for i := range cdList.Items {
		cd := &cdList.Items[i]
		if cd.Status.Managed.Group == gk.Group &&
			cd.Status.Managed.Kind == gk.Kind {
			if cd.Spec.Chart.Version == chartVersion {
				lst = append(lst, *cd)
			}
		}
	}

	return lst, nil
}

func propagateCABundle(ctx context.Context, cli client.Client, cabundle []byte, gvr schema.GroupVersionResource, log func(string, ...any)) error {
	crd, err := crdtools.Get(ctx, cli, gvr)
	if err != nil {
		return fmt.Errorf("error getting CRD: %w", err)
	}

	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Kind:    "CustomResourceDefinition",
		Version: "v1",
	})

	if len(crd.Spec.Versions) > 1 {
		log("Updating CA bundle for CRD", "Name", crd.Name)
		err = crdtools.UpdateCABundle(crd, cabundle)
		if err != nil {
			return fmt.Errorf("error updating CA bundle: %w", err)
		}
		// Update the CRD with the new CA bundle
		err = kube.Apply(ctx, cli, crd, kube.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("error applying CRD: %w", err)
		}
	}

	// Update the mutating webhook config with the new CA bundle
	mutatingWebhookConfig := admissionregistrationv1.MutatingWebhookConfiguration{}
	err = objects.CreateK8sObject(&mutatingWebhookConfig, schema.GroupVersionResource{}, types.NamespacedName{}, MutatingWebhookPath, "caBundle", base64.StdEncoding.EncodeToString(cabundle))
	if err != nil {
		return fmt.Errorf("error creating mutating webhook config: %w", err)
	}
	log("Updating CA bundle for MutatingWebhookConfiguration", "Name", mutatingWebhookConfig.Name)
	err = kube.Apply(ctx, cli, &mutatingWebhookConfig, kube.ApplyOptions{})
	if err != nil {
		return fmt.Errorf("error applying mutating webhook config: %w", err)
	}

	return nil
}
