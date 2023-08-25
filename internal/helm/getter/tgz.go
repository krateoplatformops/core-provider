package getter

import (
	"fmt"
	"strings"
)

var _ Getter = (*tgzGetter)(nil)

type tgzGetter struct{}

func (g *tgzGetter) Get(opts GetOptions) ([]byte, error) {
	if !strings.HasSuffix(opts.URI, ".tgz") {
		return nil, fmt.Errorf("uri '%s' is not a valid .tgz ref", opts.URI)
	}

	return fetch(opts)
}
