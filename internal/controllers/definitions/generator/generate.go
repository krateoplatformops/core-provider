package generator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/krateoplatformops/core-provider/internal/controllers/definitions/generator/code"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	tmpModPathFmt = "github.com/krateoplatformops/%s"
	defaultGroup  = "composition.krateo.io"
)

type GroupVersionKindGetter interface {
	GVK() (schema.GroupVersionKind, error)
}

type ValuesSchemaGetter interface {
	ValuesSchemaBytes() ([]byte, error)
}

func Generate(ctx context.Context, rootDir string, gvkGetter GroupVersionKindGetter, valuesSchemaGetter ValuesSchemaGetter) ([]byte, error) {
	gen := &crdGenerator{rootDir: rootDir}
	return gen.generate(ctx, gvkGetter, valuesSchemaGetter)
}

type crdGenerator struct {
	rootDir string
}

func (g *crdGenerator) generate(ctx context.Context, gvkGetter GroupVersionKindGetter, valuesSchemaGetter ValuesSchemaGetter) ([]byte, error) {
	gvk, err := gvkGetter.GVK()
	if err != nil {
		return nil, err
	}

	dat, err := valuesSchemaGetter.ValuesSchemaBytes()
	if err != nil {
		return nil, err
	}

	res := code.Resource{
		Group:      gvk.Group,
		Version:    gvk.Version,
		Kind:       gvk.Kind,
		Schema:     dat,
		Categories: []string{"krateo", "composition"},
	}

	cfg, err := defaultCodeGeneratorOptions(g.rootDir)
	if err != nil {
		return nil, err
	}

	clean := len(os.Getenv("GEN_CLEAN_WORKDIR")) == 0
	if clean {
		defer os.RemoveAll(cfg.Workdir)
	}

	if err := code.Do(&res, cfg); err != nil {
		return nil, err
	}

	cmd := exec.Command("go", "mod", "init", cfg.Module)
	cmd.Dir = cfg.Workdir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: performing 'go mod init' (workdir: %s, module: %s, gvk: %s/%s,%s)",
			err.Error(), cfg.Workdir, cfg.Module, res.Group, res.Version, res.Kind)
	}

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = cfg.Workdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return nil, fmt.Errorf("%s: performing 'go mod tidy' (workdir: %s, module: %s, gvk: %s/%s,%s)",
				string(out), cfg.Workdir, cfg.Module, res.Group, res.Version, res.Kind)
		}
		return nil, fmt.Errorf("%s: performing 'go mod tidy' (workdir: %s, module: %s, gvk: %s/%s,%s)",
			err.Error(), cfg.Workdir, cfg.Module, res.Group, res.Version, res.Kind)
	}

	cmd = exec.Command("go",
		"run",
		"--tags",
		"generate",
		"sigs.k8s.io/controller-tools/cmd/controller-gen",
		"object:headerFile=./hack/boilerplate.go.txt",
		"paths=./...", "crd:crdVersions=v1",
		"output:artifacts:config=./crds",
	)
	cmd.Dir = cfg.Workdir
	out, err = cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return nil, fmt.Errorf("%s: performing 'go run --tags generate...' (workdir: %s, module: %s, gvk: %s/%s,%s)",
				string(out), cfg.Workdir, cfg.Module, res.Group, res.Version, res.Kind)
		}
		return nil, fmt.Errorf("%s: performing 'go run --tags generate...' (workdir: %s, module: %s, gvk: %s/%s,%s)",
			err.Error(), cfg.Workdir, cfg.Module, res.Group, res.Version, res.Kind)
	}

	fsys := os.DirFS(cfg.Workdir)
	all, err := fs.ReadDir(fsys, "crds")
	if err != nil {
		return nil, err
	}

	fp, err := fsys.Open(filepath.Join("crds", all[0].Name()))
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	return io.ReadAll(fp)
}

func defaultCodeGeneratorOptions(rootDir string) (opts code.Options, err error) {
	opts.Module = fmt.Sprintf(tmpModPathFmt, rootDir)
	opts.Workdir = filepath.Join(os.TempDir(), opts.Module)
	err = os.MkdirAll(opts.Workdir, os.ModePerm)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return opts, err
		}
	}

	return opts, nil
}
