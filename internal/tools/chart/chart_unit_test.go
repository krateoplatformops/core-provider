package chart

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/plumbing/helm/getter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestChartInfoFromSpecRetriesTransientFailures(t *testing.T) {
	origGetter := chartGetter
	origWait := chartRetryWait
	t.Cleanup(func() {
		chartGetter = origGetter
		chartRetryWait = origWait
	})
	chartRetryWait = func(context.Context, time.Duration) error { return nil }

	chartBytes := mustChartArchive(t, "demo-chart")

	t.Run("nil reader then success", func(t *testing.T) {
		attempts := 0
		chartGetter = func(context.Context, string, ...getter.Option) (io.Reader, string, error) {
			attempts++
			if attempts == 1 {
				return nil, "", nil
			}

			return bytes.NewReader(chartBytes), "", nil
		}

		pkg, rootDir, err := ChartInfoFromSpec(context.Background(), nil, &v1alpha1.ChartInfo{
			Url: "oci://example.invalid/chart",
		})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
		if rootDir != "demo-chart" {
			t.Fatalf("expected root dir demo-chart, got %q", rootDir)
		}
		if pkg == nil {
			t.Fatal("expected chart package, got nil")
		}
	})

	t.Run("getter error then success", func(t *testing.T) {
		attempts := 0
		chartGetter = func(context.Context, string, ...getter.Option) (io.Reader, string, error) {
			attempts++
			if attempts == 1 {
				return nil, "", errors.New("storage is (re)initializing")
			}

			return bytes.NewReader(chartBytes), "", nil
		}

		pkg, rootDir, err := ChartInfoFromSpec(context.Background(), nil, &v1alpha1.ChartInfo{
			Url: "oci://example.invalid/chart",
		})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
		if rootDir != "demo-chart" {
			t.Fatalf("expected root dir demo-chart, got %q", rootDir)
		}
		if pkg == nil {
			t.Fatal("expected chart package, got nil")
		}
	})

	t.Run("unknown getter error then success", func(t *testing.T) {
		attempts := 0
		chartGetter = func(context.Context, string, ...getter.Option) (io.Reader, string, error) {
			attempts++
			if attempts == 1 {
				return nil, "", errors.New("boom")
			}

			return bytes.NewReader(chartBytes), "", nil
		}

		pkg, rootDir, err := ChartInfoFromSpec(context.Background(), nil, &v1alpha1.ChartInfo{
			Url: "oci://example.invalid/chart",
		})
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
		if rootDir != "demo-chart" {
			t.Fatalf("expected root dir demo-chart, got %q", rootDir)
		}
		if pkg == nil {
			t.Fatal("expected chart package, got nil")
		}
	})
}

func TestChartInfoFromSpecFailsFastOnDenylistedError(t *testing.T) {
	origGetter := chartGetter
	origWait := chartRetryWait
	t.Cleanup(func() {
		chartGetter = origGetter
		chartRetryWait = origWait
	})
	chartRetryWait = func(context.Context, time.Duration) error { return nil }

	attempts := 0
	chartGetter = func(context.Context, string, ...getter.Option) (io.Reader, string, error) {
		attempts++
		return nil, "", apierrors.NewForbidden(schema.GroupResource{Group: "source.toolkit.fluxcd.io", Resource: "helmcharts"}, "demo-chart", errors.New("forbidden"))
	}

	_, _, err := ChartInfoFromSpec(context.Background(), nil, &v1alpha1.ChartInfo{
		Url: "oci://example.invalid/chart",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestIsRetryableChartError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "forbidden", err: apierrors.NewForbidden(schema.GroupResource{Group: "g", Resource: "r"}, "name", errors.New("forbidden")), want: false},
		{name: "bad request", err: apierrors.NewBadRequest("bad request"), want: false},
		{name: "not found", err: apierrors.NewNotFound(schema.GroupResource{Group: "g", Resource: "r"}, "name"), want: false},
		{name: "unknown", err: errors.New("boom"), want: true},
		{name: "storage init", err: errors.New("storage is (re)initializing"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableChartError(tt.err); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func mustChartArchive(t *testing.T, rootDir string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	chartYAML := []byte("apiVersion: v2\nname: demo-chart\nversion: 1.2.3\n")
	hdr := &tar.Header{
		Name:    rootDir + "/Chart.yaml",
		Mode:    0o644,
		Size:    int64(len(chartYAML)),
		ModTime: time.Unix(0, 0),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(chartYAML); err != nil {
		t.Fatalf("failed to write chart bytes: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}
