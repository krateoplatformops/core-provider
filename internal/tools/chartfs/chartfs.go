package chartfs

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgz"
	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/tgzfs"
)

func FromFile(filename string) (*ChartFS, error) {
	fin, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fin.Close()

	pkg, err := tgzfs.New(fin)
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
		return nil, fmt.Errorf("archive '%s' should contain only one root dir", filename)
	}

	return &ChartFS{
		rootdir: dirs[0],
		fs:      pkg,
	}, nil
}

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

	dirs := []string{}
	for _, el := range all {
		if el.IsDir() {
			dirs = append(dirs, el.Name())
		}
	}

	if len(dirs) != 1 {
		return nil, fmt.Errorf("archive '%s' should contain only one root dir", url)
	}

	return &ChartFS{
		rootdir: dirs[0],
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
