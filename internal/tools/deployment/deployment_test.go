package deployment_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/krateoplatformops/core-provider/internal/tools/deployment"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRestartDeployment(t *testing.T) {
	// Setup
	ctx := context.TODO()
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)

	deploymentObj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploymentObj).Build()

	// Act
	err := deployment.RestartDeployment(ctx, client, deploymentObj)

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, deploymentObj.Spec.Template.Annotations, "kubectl.kubernetes.io/restartedAt")
	_, err = time.Parse(time.RFC3339, deploymentObj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"])
	assert.NoError(t, err)
}

func TestCleanFromRestartAnnotation(t *testing.T) {
	// Setup
	deploymentObj := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	// Act
	deployment.CleanFromRestartAnnotation(deploymentObj)

	// Assert
	assert.NotContains(t, deploymentObj.Spec.Template.Annotations, "kubectl.kubernetes.io/restartedAt")
}
