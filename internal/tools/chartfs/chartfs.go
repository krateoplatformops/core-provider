package chartfs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"

	"github.com/krateoplatformops/core-provider/apis/definitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgzfs"
	"github.com/krateoplatformops/core-provider/internal/helm/getter"
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

func ForSpec(nfo *v1alpha1.ChartInfo) (*ChartFS, error) {
	if nfo == nil {
		return nil, fmt.Errorf("chart infos cannot be nil")
	}

	dat, err := getter.Get(getter.GetOptions{
		URI:     nfo.Url,
		Version: nfo.Version,
		Name:    nfo.Name,
	})
	if err != nil {
		return nil, err
	}

	return FromReader(bytes.NewBuffer(dat))
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
