package compositiondefinitions

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	contexttools "github.com/krateoplatformops/core-provider/internal/tools/context"
	"github.com/krateoplatformops/core-provider/internal/tools/retry"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	compositionDefinitionRetryAttempts     = 5
	compositionDefinitionRetryInitialDelay = 250 * time.Millisecond
	compositionDefinitionRetryMaximumDelay = 2 * time.Second
)

var compositionDefinitionRetryWait = retry.Wait

func (e *external) updateCompositionDefinitionStatusWithRetry(ctx context.Context, namespace, name string, mutate func(*compositiondefinitionsv1alpha1.CompositionDefinition)) error {
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())
	key := client.ObjectKey{Namespace: namespace, Name: name}

	_, err := retry.Do[struct{}](ctx, retry.Config[struct{}]{
		Attempts:     compositionDefinitionRetryAttempts,
		InitialDelay: compositionDefinitionRetryInitialDelay,
		MaximumDelay: compositionDefinitionRetryMaximumDelay,
		Wait:         compositionDefinitionRetryWait,
		Retryable:    isRetryableCompositionDefinitionError,
		OnRetry: func(attempt int, nextDelay time.Duration, err error) {
			log.Warn("Retrying CompositionDefinition status update", "name", name, "namespace", namespace, "attempt", attempt, "next_delay", nextDelay, "error", err)
		},
	}, func(context.Context) (struct{}, error) {
		current := &compositiondefinitionsv1alpha1.CompositionDefinition{}
		if err := e.kube.Get(ctx, key, current); err != nil {
			return struct{}{}, err
		}

		before := current.Status.DeepCopy()
		mutate(current)
		if equality.Semantic.DeepEqual(*before, current.Status) {
			return struct{}{}, nil
		}

		if err := e.kube.Status().Update(ctx, current); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	})
	if err != nil {
		return fmt.Errorf("updating CompositionDefinition status %s/%s failed after %d attempts: %w", namespace, name, compositionDefinitionRetryAttempts, err)
	}

	return nil
}

func (e *external) updateCompositionDefinitionWithRetry(ctx context.Context, namespace, name string, mutate func(*compositiondefinitionsv1alpha1.CompositionDefinition) bool) error {
	log := contexttools.LoggerFromCtx(ctx, logging.NewNopLogger())
	key := client.ObjectKey{Namespace: namespace, Name: name}

	_, err := retry.Do[struct{}](ctx, retry.Config[struct{}]{
		Attempts:     compositionDefinitionRetryAttempts,
		InitialDelay: compositionDefinitionRetryInitialDelay,
		MaximumDelay: compositionDefinitionRetryMaximumDelay,
		Wait:         compositionDefinitionRetryWait,
		Retryable:    isRetryableCompositionDefinitionError,
		OnRetry: func(attempt int, nextDelay time.Duration, err error) {
			log.Warn("Retrying CompositionDefinition update", "name", name, "namespace", namespace, "attempt", attempt, "next_delay", nextDelay, "error", err)
		},
	}, func(context.Context) (struct{}, error) {
		current := &compositiondefinitionsv1alpha1.CompositionDefinition{}
		if err := e.kube.Get(ctx, key, current); err != nil {
			if apierrors.IsNotFound(err) {
				return struct{}{}, nil
			}
			return struct{}{}, err
		}

		if !mutate(current) {
			return struct{}{}, nil
		}

		if err := e.kube.Update(ctx, current); err != nil {
			if apierrors.IsNotFound(err) {
				return struct{}{}, nil
			}
			return struct{}{}, err
		}

		return struct{}{}, nil
	})
	if err != nil {
		return fmt.Errorf("updating CompositionDefinition %s/%s failed after %d attempts: %w", namespace, name, compositionDefinitionRetryAttempts, err)
	}

	return nil
}

func isRetryableCompositionDefinitionError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if isNonRetryableCompositionDefinitionError(err) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	return true
}

func isNonRetryableCompositionDefinitionError(err error) bool {
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
