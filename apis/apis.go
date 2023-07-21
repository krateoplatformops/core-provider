package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	definitions "github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
)

func init() {
	AddToSchemes = append(AddToSchemes,
		definitions.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
