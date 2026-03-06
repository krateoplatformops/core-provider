package chartfs

import (
	"context"
	"fmt"
	"io"
	"io/fs"

	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/tools/resolvers"
	"github.com/krateoplatformops/core-provider/internal/tools/tgzfs"
	"github.com/krateoplatformops/plumbing/helm/getter"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func FromReader(in io.Reader, pkgurl string) (*ChartFS, error) {
	pkg, err := tgzfs.New(in)
	if err != nil {
		return nil, err
	}

	all, err := fs.ReadDir(pkg, ".")
	if err != nil {
		return nil, err
	}

	dirs := []string{}
	for _, el := range all {
		if el.IsDir() {
			dirs = append(dirs, el.Name())
		}
	}

	if len(dirs) != 1 {
		return nil, fmt.Errorf("archive should contain only one root dir")
	}

	return &ChartFS{
		packageURL: pkgurl,
		rootdir:    dirs[0],
		fs:         pkg,
	}, nil
}

func ForSpec(ctx context.Context, kube client.Client, nfo *v1alpha1.ChartInfo) (*ChartFS, error) {
	if nfo == nil {
		return nil, fmt.Errorf("chart infos cannot be nil")
	}

	opts := []getter.Option{
		getter.WithRepo(nfo.Repo),
		getter.WithVersion(nfo.Version),
		getter.WithInsecureSkipVerifyTLS(nfo.InsecureSkipVerifyTLS),
	}
	if nfo.Credentials != nil {
		secret, err := resolvers.GetSecret(ctx, kube, nfo.Credentials.PasswordRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}
		opts = append(opts, getter.WithCredentials(nfo.Credentials.Username, secret))
	}
	dat, url, err := getter.Get(ctx, nfo.Url,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return FromReader(dat, url)
}

var _ fs.FS = (*ChartFS)(nil)

type ChartFS struct {
	packageURL string
	rootdir    string
	fs         fs.FS
}

func (c *ChartFS) PackageURL() string {
	return c.packageURL
}

func (c *ChartFS) Open(name string) (fs.File, error) {
	return c.fs.Open(name)
}

func (c *ChartFS) RootDir() string {
	return c.rootdir
}

func (c *ChartFS) FS() fs.FS {
	return c.fs
}
