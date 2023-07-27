package chartfs

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"

	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgz"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgzfs"
)

func FromURL(url string) (*ChartFS, error) {
	bin, err := tgz.Fetch(context.Background(), url)
	if err != nil {
		return nil, err
	}

	pkg, err := tgzfs.New(bytes.NewReader(bin))
	if err != nil {
		return nil, err
	}

	all, err := fs.ReadDir(pkg, ".")
	if err != nil {
		return nil, err
	}

	if len(all) != 1 {
		return nil, fmt.Errorf("archive '%s' should contain only one root dir", url)
	}

	return &ChartFS{
		rootdir: all[0].Name(),
		fs:      pkg,
	}, nil
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
