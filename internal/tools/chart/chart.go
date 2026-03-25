package chart

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/resolvers"
	"github.com/krateoplatformops/core-provider/internal/tools/strutil"
	"github.com/krateoplatformops/core-provider/internal/tools/tgzfs"
	"github.com/krateoplatformops/plumbing/helm/getter"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	defaultGroup = "composition.krateo.io"
)

var chartGetter = getter.Get

func ChartInfoFromSpec(ctx context.Context, kube client.Client, nfo *v1alpha1.ChartInfo) (pkg fs.FS, rootDir string, err error) {
	if nfo == nil {
		return nil, "", fmt.Errorf("chart infos cannot be nil")
	}
	opts := []getter.Option{
		getter.WithRepo(nfo.Repo),
		getter.WithVersion(nfo.Version),
		getter.WithInsecureSkipVerifyTLS(nfo.InsecureSkipVerifyTLS),
	}
	if nfo.Credentials != nil {
		secret, err := resolvers.GetSecret(ctx, kube, nfo.Credentials.PasswordRef)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get secret: %w", err)
		}
		opts = append(opts, getter.WithCredentials(nfo.Credentials.Username, secret))
	}
	dat, _, err := chartGetter(ctx, nfo.Url,
		opts...,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get chart: %w", err)
	}
	if dat == nil {
		return nil, "", fmt.Errorf("failed to get chart: empty response reader")
	}

	bData, err := io.ReadAll(dat)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read chart: %w", err)
	}

	return ChartInfoFromBytes(ctx, bData)
}

func ChartInfoFromBytes(ctx context.Context, bin []byte) (pkg fs.FS, rootDir string, err error) {
	pkg, err = tgzfs.New(bytes.NewBuffer(bin))
	if err != nil {
		return nil, "", err
	}

	all, err := fs.ReadDir(pkg, ".")
	if err != nil {
		return nil, "", err
	}

	if len(all) != 1 {
		return nil, "", fmt.Errorf("tgz archive should contain only one root dir")
	}

	rootDir = all[0].Name()

	return pkg, rootDir, nil
}

func ChartGroupVersionKind(tgzFS fs.FS, rootDir string) (schema.GroupVersionKind, error) {
	fin, err := tgzFS.Open(rootDir + "/Chart.yaml")
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	defer fin.Close()

	din, err := io.ReadAll(fin)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	res := map[string]any{}
	if err := yaml.Unmarshal(din, &res); err != nil {
		return schema.GroupVersionKind{}, err
	}

	return schema.GroupVersionKind{
		Group:   defaultGroup,
		Version: fmt.Sprintf("v%s", strings.ReplaceAll(res["version"].(string), ".", "-")),
		Kind:    flect.Pascalize(strutil.ToGolangName(res["name"].(string))),
	}, nil
}

func ChartJsonSchema(tgzFS fs.FS, rootDir string) ([]byte, error) {
	fin, err := tgzFS.Open(rootDir + "/values.schema.json")
	if err != nil {
		return nil, err
	}
	defer fin.Close()

	return io.ReadAll(fin)
}
