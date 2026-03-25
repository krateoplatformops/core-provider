package chart

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/plumbing/helm/getter"
)

func TestChartInfoFromSpecHandlesGetterFailures(t *testing.T) {
	origGetter := chartGetter
	t.Cleanup(func() {
		chartGetter = origGetter
	})

	tests := []struct {
		name      string
		getter    func(context.Context, string, ...getter.Option) (io.Reader, string, error)
		wantError string
	}{
		{
			name: "nil reader",
			getter: func(context.Context, string, ...getter.Option) (io.Reader, string, error) {
				return nil, "", nil
			},
			wantError: "empty response reader",
		},
		{
			name: "getter error",
			getter: func(context.Context, string, ...getter.Option) (io.Reader, string, error) {
				return nil, "", errors.New("boom")
			},
			wantError: "failed to get chart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartGetter = func(ctx context.Context, uri string, opts ...getter.Option) (io.Reader, string, error) {
				return tt.getter(ctx, uri, opts...)
			}

			_, _, err := ChartInfoFromSpec(context.Background(), nil, &v1alpha1.ChartInfo{
				Url: "oci://example.invalid/chart",
			})
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error to contain %q, got %v", tt.wantError, err)
			}
		})
	}
}
