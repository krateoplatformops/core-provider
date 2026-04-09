package kube

import (
	"context"
	"errors"
	"time"

	"github.com/krateoplatformops/core-provider/internal/tools/retry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clientRetryAttempts     = 10
	clientRetryInitialDelay = 100 * time.Millisecond
	clientRetryMaximumDelay = 100 * time.Millisecond
)

// ApplyGenericObject applies a generic object to the cluster
func Apply(ctx context.Context, kube client.Client, obj client.Object, opts ApplyOptions) error {
	_, err := retry.Do[struct{}](ctx, retry.Config[struct{}]{
		Attempts:     clientRetryAttempts,
		InitialDelay: clientRetryInitialDelay,
		MaximumDelay: clientRetryMaximumDelay,
		Retryable:    isRetryableClientError,
	}, func(context.Context) (struct{}, error) {
		tmp := &unstructured.Unstructured{}
		tmp.SetKind(obj.GetObjectKind().GroupVersionKind().Kind)
		tmp.SetAPIVersion(obj.GetObjectKind().GroupVersionKind().GroupVersion().String())
		err := kube.Get(ctx, client.ObjectKeyFromObject(obj), tmp)
		if err != nil {
			if apierrors.IsNotFound(err) {
				createOpts := &client.CreateOptions{
					DryRun:          opts.DryRun,
					FieldManager:    opts.FieldManager,
					FieldValidation: opts.FieldValidation,
				}
				return struct{}{}, kube.Create(ctx, obj, createOpts)
			}
			return struct{}{}, err
		}

		obj.SetResourceVersion(tmp.GetResourceVersion())
		updateOpts := &client.UpdateOptions{
			DryRun:          opts.DryRun,
			FieldManager:    opts.FieldManager,
			FieldValidation: opts.FieldValidation,
		}
		return struct{}{}, kube.Update(ctx, obj, updateOpts)
	})
	return err
}

func Uninstall(ctx context.Context, kube client.Client, obj client.Object, opts UninstallOptions) error {
	_, err := retry.Do[struct{}](ctx, retry.Config[struct{}]{
		Attempts:     clientRetryAttempts,
		InitialDelay: clientRetryInitialDelay,
		MaximumDelay: clientRetryMaximumDelay,
		Retryable:    isRetryableClientError,
	}, func(context.Context) (struct{}, error) {
		tmp := &unstructured.Unstructured{}
		tmp.SetKind(obj.GetObjectKind().GroupVersionKind().Kind)
		tmp.SetAPIVersion(obj.GetObjectKind().GroupVersionKind().GroupVersion().String())
		err := kube.Get(ctx, client.ObjectKeyFromObject(obj), tmp)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return struct{}{}, nil
			}

			return struct{}{}, err
		}

		obj.SetResourceVersion(tmp.GetResourceVersion())

		err = kube.Delete(ctx, obj, &client.DeleteOptions{
			DryRun:             opts.DryRun,
			Preconditions:      opts.Preconditions,
			PropagationPolicy:  opts.PropagationPolicy,
			GracePeriodSeconds: opts.GracePeriodSeconds,
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return struct{}{}, nil
			}

			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func Get(ctx context.Context, kube client.Client, obj client.Object) error {
	return kube.Get(ctx, client.ObjectKeyFromObject(obj), obj)
}

func isRetryableClientError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	return true
}
