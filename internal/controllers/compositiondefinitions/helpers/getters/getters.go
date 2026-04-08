package getters

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	contexttools "github.com/krateoplatformops/core-provider/internal/tools/context"
	"github.com/krateoplatformops/core-provider/internal/tools/retry"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/deploy"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	compositionRetryAttempts     = 5
	compositionRetryInitialDelay = 250 * time.Millisecond
	compositionRetryMaximumDelay = 2 * time.Second
)

var retryWait = retry.Wait

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

	ul, err := getCompositionsWithRetry(ctx, dyn, gvr, log)
	if err != nil {
		return fmt.Errorf("error getting compositions: %w", err)
	}

	if len(ul.Items) == 0 {
		log.Debug("No compositions found for the specified GVR and version")
		return nil
	}

	for _, u := range ul.Items {
		if err := updateCompositionWithRetry(ctx, dyn, gvr, u.GetNamespace(), u.GetName(), newVersion, log); err != nil {
			return err
		}
	}

	return nil
}

func getCompositionsWithRetry(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, log logging.Logger) (*unstructured.UnstructuredList, error) {
	ul, err := retry.Do[*unstructured.UnstructuredList](ctx, retry.Config[*unstructured.UnstructuredList]{
		Attempts:     compositionRetryAttempts,
		InitialDelay: compositionRetryInitialDelay,
		MaximumDelay: compositionRetryMaximumDelay,
		Wait:         retryWait,
		Retryable:    isRetryableCompositionError,
		OnRetry: func(attempt int, nextDelay time.Duration, err error) {
			log.Warn("Retrying composition list", "gvr", gvr.String(), "attempt", attempt, "next_delay", nextDelay, "error", err)
		},
	}, func(context.Context) (*unstructured.UnstructuredList, error) {
		return GetCompositions(ctx, dyn, gvr)
	})
	if err != nil {
		return nil, fmt.Errorf("error listing compositions after %d attempts: %w", compositionRetryAttempts, err)
	}

	return ul, nil
}

func updateCompositionWithRetry(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, namespace, name, newVersion string, log logging.Logger) error {
	compositionName := compositionKey(namespace, name)
	_, err := retry.Do[struct{}](ctx, retry.Config[struct{}]{
		Attempts:     compositionRetryAttempts,
		InitialDelay: compositionRetryInitialDelay,
		MaximumDelay: compositionRetryMaximumDelay,
		Wait:         retryWait,
		Retryable:    isRetryableCompositionError,
		OnRetry: func(attempt int, nextDelay time.Duration, err error) {
			log.Warn("Retrying composition update", "composition", compositionName, "gvr", gvr.String(), "attempt", attempt, "next_delay", nextDelay, "error", err)
		},
	}, func(context.Context) (struct{}, error) {
		u, err := dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			log.Debug("Composition disappeared before version update, skipping", "composition", compositionName, "gvr", gvr.String())
			return struct{}{}, nil
		}
		if err != nil {
			return struct{}{}, fmt.Errorf("error getting composition %s: %w", compositionName, err)
		}

		labelmap, ok, err := unstructured.NestedStringMap(u.Object, "metadata", "labels")
		if err != nil {
			return struct{}{}, fmt.Errorf("error getting labels from composition %s: %w", compositionName, err)
		}
		if !ok {
			labelmap = make(map[string]string)
		}
		if labelmap[deploy.CompositionVersionLabel] == newVersion {
			return struct{}{}, nil
		}

		labelmap[deploy.CompositionVersionLabel] = newVersion
		if err := unstructured.SetNestedStringMap(u.Object, labelmap, "metadata", "labels"); err != nil {
			return struct{}{}, fmt.Errorf("error setting labels on composition %s: %w", compositionName, err)
		}

		if _, err := dyn.Resource(gvr).Namespace(namespace).Update(ctx, u, metav1.UpdateOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				log.Debug("Composition disappeared during version update, skipping", "composition", compositionName, "gvr", gvr.String())
				return struct{}{}, nil
			}
			return struct{}{}, err
		}

		return struct{}{}, nil
	})
	if err != nil {
		return fmt.Errorf("composition %s update failed after %d attempts: %w", compositionName, compositionRetryAttempts, err)
	}

	return nil
}

func compositionKey(namespace, name string) string {
	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}

func isRetryableCompositionError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if isNonRetryableCompositionError(err) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	return true
}

func isNonRetryableCompositionError(err error) bool {
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) || apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
		return true
	}
	if statusErr, ok := err.(*apierrors.StatusError); ok {
		switch statusErr.ErrStatus.Code {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusBadRequest:
			return true
		}
	}

	return false
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
