package getter

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GetOptions struct {
	URI                    string
	Version                string
	Repo                   string
	InsecureSkipVerifyTLS  bool
	Username               string
	Password               string
	PassCredentialsAll     bool
	HelmRegistryConfigPath string
}

// Getter is an interface to support GET to the specified URI.
type Getter interface {
	// Get file content by url string
	Get(opts GetOptions) ([]byte, string, error)
}

func Get(opts GetOptions) ([]byte, string, error) {
	if isOCI(opts.URI) {
		g, err := newOCIGetter(opts.HelmRegistryConfigPath)
		if err != nil {
			return nil, "", err
		}
		return g.Get(opts)
	}

	if isTGZ(opts.URI) {
		g := &tgzGetter{}
		return g.Get(opts)
	}

	if isHTTP(opts.URI) {
		g := &repoGetter{}
		return g.Get(opts)
	}

	return nil, "", fmt.Errorf("no handler found for url: %s", opts.URI)
}

func fetch(opts GetOptions) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, opts.URI, nil)
	if err != nil {
		return nil, err
	}
	// Host on URL (returned from url.Parse) contains the port if present.
	// This check ensures credentials are not passed between different
	// services on different ports.
	if opts.PassCredentialsAll {
		if opts.Username != "" && opts.Password != "" {
			req.SetBasicAuth(opts.Username, opts.Password)
		}
	}

	// out, err := httputil.DumpRequest(req, true)
	// fmt.Println(string(out))

	resp, err := newHTTPClient(opts).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s : %s", opts.URI, resp.Status)
	}

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	return buf.Bytes(), err
}

func newHTTPClient(opts GetOptions) *http.Client {
	transport := &http.Transport{
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}

	if opts.InsecureSkipVerifyTLS {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		} else {
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   1 * time.Minute,
		// CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// 	fmt.Printf("redir: %v\n", via)
		// 	return nil
		// },
	}
}
