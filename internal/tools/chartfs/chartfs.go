package chartfs

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgzfs"
	"github.com/krateoplatformops/core-provider/internal/tools/getter"
	"helm.sh/helm/v3/pkg/registry"
)

func FromReader(in io.Reader) (*ChartFS, error) {
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
		rootdir: dirs[0],
		fs:      pkg,
	}, nil
}

func FromURL(url string) (*ChartFS, error) {
	if registry.IsOCI(url) {
		g, err := getter.NewOCIGetter()
		if err != nil {
			return nil, err
		}

		buf, err := g.Get(url)
		if err != nil {
			return nil, err
		}

		return FromReader(buf)
	}

	buf, err := getter.NewHTTPGetter().Get(url)
	if err != nil {
		return nil, err
	}

	return FromReader(buf)
}

var _ fs.FS = (*ChartFS)(nil)

type ChartFS struct {
	rootdir string
	fs      fs.FS
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
