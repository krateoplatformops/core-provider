package getters

import (
	"context"
	"fmt"

	contexttools "github.com/krateoplatformops/core-provider/internal/tools/context"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetCompositions(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())
	// Create a label requirement for the composition version
	labelreq, err := labels.NewRequirement(deploy.CompositionVersionLabel, selection.Equals, []string{gvr.Version})
	if err != nil {
		log.Debug("Error creating label requirement", "error", err)
		return nil, fmt.Errorf("error creating label requirement: %w", err)
	}
	selector := labels.NewSelector()
	selector = selector.Add(*labelreq)

	ul, err := dyn.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		log.Debug("Error listing compositions", "error", err)
		return nil, fmt.Errorf("error listing compositions: %w", err)
	}

	return ul, nil
}

// updateCompositionsVersion updates the version label of all compositions in a namespace
// that match the specified GroupVersionResource (GVR) and current version.
func UpdateCompositionsVersion(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, newVersion string) error {
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())

	ul, err := GetCompositions(ctx, dyn, gvr)
	if err != nil {
		return fmt.Errorf("error getting compositions: %w", err)
	}

	if len(ul.Items) == 0 {
		log.Debug("No compositions found for the specified GVR and version")
		return nil
	}

	for _, u := range ul.Items {
		labelmap, ok, err := unstructured.NestedStringMap(u.Object, "metadata", "labels")
		if err != nil {
			return fmt.Errorf("error getting labels from composition: %s", err)
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

func GetCompositionDefinitions(ctx context.Context, cli client.Client, gk schema.GroupKind) ([]compositiondefinitionsv1alpha1.CompositionDefinition, error) {
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err := cli.List(ctx, &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return nil, fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}

	lst := []compositiondefinitionsv1alpha1.CompositionDefinition{}
	for i := range cdList.Items {
		cd := &cdList.Items[i]

		cdgvk := schema.FromAPIVersionAndKind(cd.Status.ApiVersion, cd.Status.Kind)
		if cdgvk.Group == gk.Group &&
			cdgvk.Kind == gk.Kind {
			lst = append(lst, *cd)
		}

		// if cd.Status.Managed.Group == gk.Group &&
		// 	cd.Status.Managed.Kind == gk.Kind {
		// 	lst = append(lst, *cd)
		// }
	}

	return lst, nil
}

// GetCompositionDefinitionsWithVersion retrieves CompositionDefinitions that match the specified Composition GVK
func GetCompositionDefinitionsWithVersion(ctx context.Context, cli client.Client, gvk schema.GroupVersionKind) ([]compositiondefinitionsv1alpha1.CompositionDefinition, error) {
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err := cli.List(ctx, &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return nil, fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}

	lst := []compositiondefinitionsv1alpha1.CompositionDefinition{}
	for i := range cdList.Items {
		cd := &cdList.Items[i]
		cdgvk := schema.FromAPIVersionAndKind(cd.Status.ApiVersion, cd.Status.Kind)
		if cdgvk.Group == gvk.Group && cdgvk.Kind == gvk.Kind && cdgvk.Version == gvk.Version {
			lst = append(lst, *cd)
		}
	}

	return lst, nil
}
