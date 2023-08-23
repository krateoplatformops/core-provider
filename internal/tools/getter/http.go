package getter

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// NewHTTPGetter constructs a valid http/https client as a Getter
func NewHTTPGetter() Getter {
	return &httpGetter{
		transport: &http.Transport{
			DisableCompression: true,
			Proxy:              http.ProxyFromEnvironment,
		},
	}
}

var _ Getter = (*httpGetter)(nil)

// httpGetter is the default HTTP(/S) backend handler
type httpGetter struct {
	opts      options
	transport *http.Transport
}

// Get performs a Get from repo.Getter and returns the body.
func (g *httpGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&g.opts)
	}
	return g.get(href)
}

func (g *httpGetter) get(href string) (*bytes.Buffer, error) {
	// Set a helm specific user agent so that a repo server and metrics can
	// separate helm calls from other tools interacting with repos.
	req, err := http.NewRequest(http.MethodGet, href, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "krateo")

	// Host on URL (returned from url.Parse) contains the port if present.
	// This check ensures credentials are not passed between different
	// services on different ports.
	if g.opts.passCredentialsAll {
		if g.opts.username != "" && g.opts.password != "" {
			req.SetBasicAuth(g.opts.username, g.opts.password)
		}
	}

	client, err := g.httpClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to fetch %s : %s", href, resp.Status)
	}

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	return buf, err
}

func (g *httpGetter) httpClient() (*http.Client, error) {
	if g.opts.insecureSkipVerifyTLS {
		if g.transport.TLSClientConfig == nil {
			g.transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			g.transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	client := &http.Client{
		Transport: g.transport,
		Timeout:   1 * time.Minute,
	}

	return client, nil
}
