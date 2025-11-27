package watcher

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ReadyChecker[T client.Object] func(obj T) bool

type Watcher[T client.Object] struct {
	client  dynamic.Interface
	gvr     schema.GroupVersionResource
	timeout time.Duration
	checker ReadyChecker[T]
}

func NewWatcher[T client.Object](client dynamic.Interface, gvr schema.GroupVersionResource, timeout time.Duration, checker ReadyChecker[T]) Watcher[T] {
	return Watcher[T]{
		client:  client,
		gvr:     gvr,
		timeout: timeout,
		checker: checker,
	}
}

func (w Watcher[T]) WatchResource(ctx context.Context, namespace, name string) error {
	ctxWatch, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	watcher, err := w.client.Resource(w.gvr).Namespace(namespace).Watch(ctxWatch, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	})
	if err != nil {
		return fmt.Errorf("error watching resource %s/%s: %w", namespace, name, err)
	}
	defer watcher.Stop()

	errCh := make(chan error, 1)
	readyCh := make(chan struct{})

	go func() {
		for event := range watcher.ResultChan() {
			unstrObj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			var typedObj T
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.Object, &typedObj)
			if err != nil {
				continue
			}

			if w.checker(typedObj) {
				close(readyCh)
				return
			}

			if event.Type == "DELETED" {
				errCh <- fmt.Errorf("resource %s/%s deleted", namespace, name)
				return
			}
		}
		errCh <- fmt.Errorf("watch ended before resource %s/%s ready", namespace, name)
	}()

	select {
	case <-ctxWatch.Done():
		return fmt.Errorf("timeout waiting for resource %s/%s to be ready", namespace, name)
	case <-readyCh:
		return nil
	case err := <-errCh:
		return err
	}
}
