package deployment

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func LookupDeployment(ctx context.Context, kube client.Client, obj *appsv1.Deployment) (bool, bool, error) {
	err := kube.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}

		return false, false, err
	}

	ready := obj.Spec.Replicas != nil && *obj.Spec.Replicas == obj.Status.ReadyReplicas

	return true, ready, nil
}

func RestartDeployment(ctx context.Context, kube client.Client, obj *appsv1.Deployment) error {
	patch := client.MergeFrom(obj.DeepCopy())

	// Set the annotation to trigger a rollout
	if obj.Spec.Template.Annotations == nil {
		obj.Spec.Template.Annotations = map[string]string{}
	}
	obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	// patch the deployment
	return kube.Patch(ctx, obj, patch)
}

func CleanFromRestartAnnotation(obj *appsv1.Deployment) {
	if obj.Spec.Template.Annotations != nil {
		delete(obj.Spec.Template.Annotations, "kubectl.kubernetes.io/restartedAt")
	}
}

func IsReady(d *appsv1.Deployment) bool {
	// check ready replicas match desired replicas
	if d.Spec.Replicas != nil && d.Status.ReadyReplicas != *d.Spec.Replicas {
		return false
	}
	// check Available condition == True
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == "True" {
			return true
		}
	}
	return false
}

// func Watch(ctx context.Context, kube kubernetes.Interface, obj *appsv1.Deployment) error {
// 	ctxWatch, cancel := context.WithTimeout(ctx, 1*time.Minute)
// 	defer cancel()

// 	watcher, err := kube.AppsV1().Deployments(obj.GetNamespace()).Watch(ctxWatch, metav1.ListOptions{
// 		FieldSelector: fields.OneTermEqualSelector("metadata.name", obj.GetName()).String(),
// 	})
// 	if err != nil {
// 		return fmt.Errorf("error watching deployment %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
// 	}
// 	defer watcher.Stop()

// 	okCh := make(chan struct{})
// 	errCh := make(chan error, 1)

// 	go func() {
// 		for ev := range watcher.ResultChan() {
// 			switch ev.Type {
// 			case "ADDED", "MODIFIED":
// 				dep, castOK := ev.Object.(*appsv1.Deployment)
// 				if !castOK {
// 					// sometimes the watch returns unstructured objects depending on transport;
// 					// skip if not the correct type.
// 					continue
// 				}
// 				if IsReady(dep) {
// 					close(okCh)
// 					return
// 				}
// 			case "DELETED":
// 				// treat deleted as failure
// 				errCh <- fmt.Errorf("deployment %s/%s was deleted", obj.GetNamespace(), obj.GetName())
// 				return
// 			}
// 		}
// 		// watcher closed
// 		errCh <- fmt.Errorf("deployment watch closed before ready")
// 	}()

// 	select {
// 	case <-ctxWatch.Done():
// 		return fmt.Errorf("timeout waiting for deployment %s/%s to be ready", obj.GetNamespace(), obj.GetName())
// 	case <-okCh:
// 		// ready
// 	case err := <-errCh:
// 		return err
// 	}
// 	return nil
// }
