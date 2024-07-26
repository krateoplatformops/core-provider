package tools

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/krateoplatformops/core-provider/internal/tools/chartfs"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Names struct {
	Kind   string `json:"kind"`
	Plural string `json:"plural"`
}
type Version struct {
	Name string `json:"name"`
}

type Spec struct {
	Names    Names     `json:"names"`
	Group    string    `json:"group"`
	Versions []Version `json:"versions"`
	Scope    string    `json:"scope"`
}

type CRD struct {
	Spec Spec `json:"spec"`
}

type CRDInfo struct {
	Kind     string
	Resource string
	schema.GroupVersion
	Namespaced bool
}

func GetCRDInfoList(pkg *chartfs.ChartFS) ([]CRDInfo, error) {
	var crdList []CRDInfo

	crdsDir := path.Join(pkg.RootDir(), "crds")
	dir, err := fs.ReadDir(pkg.FS(), crdsDir)

	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", crdsDir, err)
	}

	for _, file := range dir {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			fi, _ := pkg.Open(path.Join(crdsDir, file.Name()))
			fileInfo, _ := fi.Stat()

			content := make([]byte, fileInfo.Size())
			_, err := io.ReadFull(fi, content)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %v", file.Name(), err)
			}

			crd := &CRD{}
			err = yaml.Unmarshal(content, crd)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal CRD: %v", err)
			}
			for _, version := range crd.Spec.Versions {
				gvk := CRDInfo{
					GroupVersion: schema.GroupVersion{
						Group:   crd.Spec.Group,
						Version: version.Name,
					},
					Kind:       crd.Spec.Names.Kind,
					Resource:   crd.Spec.Names.Plural,
					Namespaced: crd.Spec.Scope == "Namespaced",
				}
				crdList = append(crdList, gvk)
			}

		}
	}
	return crdList, nil
}
