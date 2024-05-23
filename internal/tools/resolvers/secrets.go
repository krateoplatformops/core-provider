package resolvers

import (
	"context"

	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetSecret(ctx context.Context, kube client.Client, secretKeySelector rtv1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{
		Name:      secretKeySelector.Name,
		Namespace: secretKeySelector.Namespace,
	}, secret); err != nil {
		return "", err
	}

	return string(secret.Data[secretKeySelector.Key]), nil
}
