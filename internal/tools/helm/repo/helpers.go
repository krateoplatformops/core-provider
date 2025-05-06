package repo

import (
	"net/url"
	"path"
)

// URLJoin joins a base URL to one or more path components.
//
// It's like filepath.Join for URLs. If the baseURL is pathish, this will still
// perform a join.
//
// If the URL is unparsable, this returns an error.
func URLJoin(baseURL string, paths ...string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	// We want path instead of filepath because path always uses /.
	all := []string{u.Path}
	all = append(all, paths...)
	u.Path = path.Join(all...)
	return u.String(), nil
}
