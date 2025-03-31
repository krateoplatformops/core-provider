package pluralizer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type names struct {
	Plural   string   `json:"plural"`
	Singular string   `json:"singular"`
	Shorts   []string `json:"shorts"`
}

var ErrNotFound = fmt.Errorf("not found")

type PluralizerInterface interface {
	GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error)
}

type Pluralizer struct {
	urlPlurals *string
	cli        *http.Client
}

func New(urlPlurals *string, cli *http.Client) *Pluralizer {
	return &Pluralizer{
		urlPlurals: urlPlurals,
		cli:        cli,
	}
}

func (p Pluralizer) GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	if p.urlPlurals == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("urlplurals is nil")
	}

	url := fmt.Sprintf("%s?kind=%s&apiVersion=%s", *p.urlPlurals, gvk.Kind, fmt.Sprintf("%s/%s", gvk.Group, gvk.Version))
	resp, err := p.cli.Get(url)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("getting urlplurals: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return schema.GroupVersionResource{}, ErrNotFound
		}
		return schema.GroupVersionResource{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	names := names{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("reading response body: %w", err)
	}
	resp.Body.Close()

	err = json.Unmarshal(body, &names)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("unmarshalling response body: %w", err)
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: names.Plural,
	}, nil
}
