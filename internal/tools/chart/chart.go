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
	"github.com/krateoplatformops/core-provider/internal/tools/helm/getter"
	"github.com/krateoplatformops/core-provider/internal/tools/resolvers"
	"github.com/krateoplatformops/core-provider/internal/tools/strutil"
	"github.com/krateoplatformops/core-provider/internal/tools/tgzfs"
	"github.com/krateoplatformops/crdgen"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	defaultGroup = "composition.krateo.io"
)

func ChartInfoFromSpec(ctx context.Context, kube client.Client, nfo *v1alpha1.ChartInfo) (pkg fs.FS, rootDir string, err error) {
	if nfo == nil {
		return nil, "", fmt.Errorf("chart infos cannot be nil")
	}

	opts := getter.GetOptions{
		URI:                   nfo.Url,
		Version:               nfo.Version,
		Repo:                  nfo.Repo,
		InsecureSkipVerifyTLS: nfo.InsecureSkipVerifyTLS,
	}
	if nfo.Credentials != nil {
		secret, err := resolvers.GetSecret(ctx, kube, nfo.Credentials.PasswordRef)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get secret: %w", err)
		}
		opts.Username = nfo.Credentials.Username
		opts.Password = secret
		opts.PassCredentialsAll = true
	}

	dat, _, err := getter.Get(opts)
	if err != nil {
		return nil, "", err
	}

	return ChartInfoFromBytes(ctx, dat)
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

func ChartJsonSchemaGetter(tgzFS fs.FS, rootDir string) crdgen.JsonSchemaGetter {
	return &chartJsonSchemaGetter{
		tgzFS: tgzFS, rootDir: rootDir,
	}
}

var _ crdgen.JsonSchemaGetter = (*chartJsonSchemaGetter)(nil)

type chartJsonSchemaGetter struct {
	tgzFS   fs.FS
	rootDir string
}

func (g *chartJsonSchemaGetter) Get() ([]byte, error) {
	fin, err := g.tgzFS.Open(g.rootDir + "/values.schema.json")
	if err != nil {
		return nil, err
	}
	defer fin.Close()

	return io.ReadAll(fin)
}
