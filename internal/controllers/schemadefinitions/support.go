package schemadefinitions

import (
	"fmt"
	"io"
	"net/http"

	"github.com/krateoplatformops/crdgen"
)

var _ crdgen.JsonSchemaGetter = (*urlJsonSchemaGetter)(nil)

func UrlJsonSchemaGetter(u string) crdgen.JsonSchemaGetter {
	return &urlJsonSchemaGetter{u}
}

type urlJsonSchemaGetter struct {
	uri string
}

func (f *urlJsonSchemaGetter) Get() ([]byte, error) {
	res, err := http.Get(f.uri)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	return io.ReadAll(http.MaxBytesReader(nil, res.Body, 512*1024))
}
