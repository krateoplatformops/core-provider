package compositiondefinitions

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/certificates"
	"github.com/krateoplatformops/core-provider/internal/tools/chart"
	"github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/kube/watcher"
	crdgen "github.com/krateoplatformops/crdgen/v2"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
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
	"digest": {
	  "type": "string",
	  "default": ""
	},
	"previousDigest": {
	  "type": "string",
	  "default": ""
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
          },
		  "path": {
            "type": "string"
          }
        }
      }
    }
  }
}`
)

// var _ crdgen.JsonSchemaGetter = (*staticJsonSchemaGetter)(nil)

func StaticJsonSchemaGetter() *staticJsonSchemaGetter {
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

// func propagateCABundle(ctx context.Context, cli client.Client, cabundle []byte, gvr schema.GroupVersionResource, log func(string, ...any)) error {
// 	if log == nil {
// 		log = func(msg string, keysAndValues ...any) {
// 			// No-op logger
// 		}
// 	}
// 	crd, err := crdtools.Get(ctx, cli, gvr.GroupResource())
// 	if err != nil {
// 		return fmt.Errorf("error getting CRD: %w", err)
// 	}

// 	crd.SetGroupVersionKind(schema.GroupVersionKind{
// 		Group:   "apiextensions.k8s.io",
// 		Kind:    "CustomResourceDefinition",
// 		Version: "v1",
// 	})

// 	if len(crd.Spec.Versions) > 1 {
// 		log("Updating CA bundle for CRD", "Name", crd.Name)
// 		err = crdtools.UpdateCABundle(crd, cabundle)
// 		if err != nil {
// 			return fmt.Errorf("error updating CA bundle: %w", err)
// 		}
// 		// Update the CRD with the new CA bundle
// 		err = kube.Apply(ctx, cli, crd, kube.ApplyOptions{})
// 		if err != nil {
// 			return fmt.Errorf("error applying CRD: %w", err)
// 		}
// 	}

// 	// Update the mutating webhook config with the new CA bundle
// 	mutatingWebhookConfig := admissionregistrationv1.MutatingWebhookConfiguration{}
// 	err = objects.CreateK8sObject(&mutatingWebhookConfig, schema.GroupVersionResource{}, types.NamespacedName{}, MutatingWebhookPath, "caBundle", base64.StdEncoding.EncodeToString(cabundle))
// 	if err != nil {
// 		return fmt.Errorf("error creating mutating webhook config: %w", err)
// 	}
// 	log("Updating CA bundle for MutatingWebhookConfiguration", "Name", mutatingWebhookConfig.Name)
// 	err = kube.Apply(ctx, cli, &mutatingWebhookConfig, kube.ApplyOptions{})
// 	if err != nil {
// 		return fmt.Errorf("error applying mutating webhook config: %w", err)
// 	}

// 	return nil
// }

func generateCRD(pkg fs.FS, dir string, gvk schema.GroupVersionKind, onlyMetadata bool) ([]byte, error) {

	var specSchema []byte
	var statusSchema []byte
	var err error

	jsonSchemaGetter := chart.ChartJsonSchemaGetter(pkg, dir)
	specSchema, err = jsonSchemaGetter.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting JSON schema: %w", err)
	}

	statusSchema, err = StaticJsonSchemaGetter().Get()
	if err != nil {
		return nil, fmt.Errorf("error getting status JSON schema: %w", err)
	}

	if onlyMetadata {
		const emptySchema = `{
  "$schema": "http://json-schema.org",
  "type": "object",
  "properties": {
		"empty": {
		  "type": "string"
		}
  }
}`
		specSchema = []byte(emptySchema)
		statusSchema = []byte(emptySchema)
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

func applyOrUpdateCRD(ctx context.Context,
	cli client.Client,
	dyn dynamic.Interface,
	newcrd *apiextensionsv1.CustomResourceDefinition,
	certMgr *certificates.CertManager,
	log func(msg string, keysAndValues ...any)) (schema.GroupVersionResource, error) {

	// Getting GVR from CRD
	gvr := schema.GroupVersionResource{
		Group:    newcrd.Spec.Group,
		Version:  newcrd.Spec.Versions[0].Name,
		Resource: newcrd.Spec.Names.Plural,
	}

	crd, err := crdtools.Get(ctx, cli, gvr.GroupResource())
	if err != nil {
		return gvr, fmt.Errorf("error getting CRD: %w", err)
	}

	if crd == nil {
		log("Creating CRD", "gvr", gvr.String())
		err = kube.Apply(ctx, cli, newcrd, kube.ApplyOptions{})
		if err != nil {
			return gvr, fmt.Errorf("error applying CRD: %w", err)
		}
		err = watcher.NewWatcher(
			dyn,
			apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"),
			1*time.Minute,
			crdtools.IsReady).WatchResource(ctx, "", newcrd.Name)
		if err != nil {
			return gvr, fmt.Errorf("error waiting for CRD to be established: %w", err)
		}

		return gvr, nil
	}
	log("Updating CRD", "gvr", gvr.String())
	versionExist, err := crdtools.Lookup(ctx, cli, gvr)
	if err != nil {
		return gvr, fmt.Errorf("error looking up CRD version: %w", err)
	}
	if versionExist {
		err = crdtools.UpdateVersion(crd, newcrd.Spec.Versions[0])
		if err != nil {
			return gvr, fmt.Errorf("error updating CRD version: %w", err)
		}
		return gvr, nil
	}

	crd, err = crdtools.AppendVersion(*crd, *newcrd)
	if err != nil {
		return gvr, fmt.Errorf("error appending version to CRD: %w", err)
	}

	certMgr.InjectConversionConfToCRD(crd)

	crdtools.SetServedStorage(crd, gvr.Version, true, false)
	err = kube.Apply(ctx, cli, crd, kube.ApplyOptions{})
	if err != nil {
		return gvr, fmt.Errorf("error setting properties on CRD: %w", err)
	}

	err = watcher.NewWatcher(
		dyn,
		apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"),
		1*time.Minute,
		crdtools.IsReady).WatchResource(ctx, "", crd.Name)
	if err != nil {
		return gvr, fmt.Errorf("error waiting for CRD to be established: %w", err)
	}

	return gvr, nil
}
