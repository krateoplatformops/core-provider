package getter

import (
	"fmt"
	"strings"

	"github.com/krateoplatformops/core-provider/internal/helm/repo"
)

var _ Getter = (*repoGetter)(nil)

type repoGetter struct{}

func (g *repoGetter) Get(opts GetOptions) ([]byte, error) {
	if !isHTTP(opts.URI) {
		return nil, fmt.Errorf("uri '%s' is not a valid Repo ref", opts.URI)
	}

	buf, err := fetch(GetOptions{
		URI:                   fmt.Sprintf("%s/index.yaml", opts.URI),
		InsecureSkipVerifyTLS: opts.InsecureSkipVerifyTLS,
		Username:              opts.Username,
		Password:              opts.Password,
		PassCredentialsAll:    opts.PassCredentialsAll,
	})
	if err != nil {
		return nil, err
	}

	idx, err := repo.Load(buf, opts.URI)
	if err != nil {
		return nil, err
	}

	res, err := idx.Get(opts.Name, opts.Version)
	if err != nil {
		return nil, err
	}
	if len(res.URLs) == 0 {
		return nil, fmt.Errorf("no package url found in index @ %s/%s", res.Name, res.Version)
	}

	return fetch(GetOptions{
		URI:                   res.URLs[0],
		Version:               res.Version,
		Name:                  res.Name,
		InsecureSkipVerifyTLS: opts.InsecureSkipVerifyTLS,
		Username:              opts.Username,
		Password:              opts.Password,
		PassCredentialsAll:    opts.PassCredentialsAll,
	})
}

func isHTTP(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}
