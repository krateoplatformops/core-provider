package repo

import (
	"log"

	"sigs.k8s.io/yaml"
)

// loadIndex loads an index file and does minimal validity checking.
//
// The source parameter is only used for logging.
// This will fail if API Version is not set (ErrNoAPIVersion) or if the unmarshal fails.
func Load(data []byte, source string) (*IndexFile, error) {
	i := &IndexFile{}

	if len(data) == 0 {
		return i, ErrEmptyIndexYaml
	}

	if err := yaml.UnmarshalStrict(data, i); err != nil {
		return i, err
	}

	for name, cvs := range i.Entries {
		for idx := len(cvs) - 1; idx >= 0; idx-- {
			if cvs[idx] == nil {
				log.Printf("skipping loading invalid entry for chart %q from %s: empty entry", name, source)
				continue
			}
			if cvs[idx].APIVersion == "" {
				cvs[idx].APIVersion = APIVersionV1
			}
		}
	}
	i.SortEntries()
	if i.APIVersion == "" {
		return i, ErrNoAPIVersion
	}
	return i, nil
}
