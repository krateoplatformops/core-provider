package getter

import (
	"fmt"
	"strings"
)

var _ Getter = (*tgzGetter)(nil)

type tgzGetter struct{}

func (g *tgzGetter) Get(opts GetOptions) ([]byte, error) {
	if !isTGZ(opts.URI) {
		return nil, fmt.Errorf("uri '%s' is not a valid .tgz ref", opts.URI)
	}

	return fetch(opts)
}

func isTGZ(url string) bool {
	return strings.HasSuffix(url, ".tgz")
}
