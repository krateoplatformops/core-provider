//go:build integration
// +build integration

package pluralizer_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gobuffalo/flect"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// FakePluralizer implements the PluralizerInterface for testing purposes.
type FakePluralizer struct{}

func (p FakePluralizer) GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: flect.Pluralize(strings.ToLower(gvk.Kind)),
	}, nil
}

var testenv env.Environment

func TestMain(m *testing.M) {
	testenv = env.New()
	testenv.Run(m)
}

func TestPluralizerIntegration(t *testing.T) {
	f := features.New("GVKtoGVR Integration Test").
		Assess("Convert GVK to GVR", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Use the FakePluralizer for testing
			pl := FakePluralizer{}

			// Test data
			gvk := schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			}

			// Call the GVKtoGVR function
			gvr, err := pl.GVKtoGVR(gvk)

			// Assertions
			assert.NoError(t, err)
			assert.Equal(t, schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			}, gvr)

			return ctx
		}).Feature()

	testenv.Test(t, f)
}
