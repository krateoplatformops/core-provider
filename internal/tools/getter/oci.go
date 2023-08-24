package getter

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/registry"
)

func NewOCIGetter() (Getter, error) {
	transport := &http.Transport{
		// From https://github.com/google/go-containerregistry/blob/31786c6cbb82d6ec4fb8eb79cd9387905130534e/pkg/v1/remote/options.go#L87
		DisableCompression: true,
		DialContext: (&net.Dialer{
			// By default we wrap the transport in retries, so reduce the
			// default dial timeout to 5s to avoid 5x 30s of connection
			// timeouts when doing the "ping" on certain http registries.
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 3 * time.Second,
	}

	client, err := registry.NewClient(
		registry.ClientOptDebug(true),
		registry.ClientOptHTTPClient(&http.Client{
			Transport: transport,
			//Timeout:   g.opts.timeout,
		}),
	)
	if err != nil {
		return nil, err
	}

	return &ociGetter{
		client: client,
	}, nil
}

var _ Getter = (*ociGetter)(nil)

type ociGetter struct {
	opts   options
	client *registry.Client
}

func (g *ociGetter) Get(uri string, opts ...Option) (*bytes.Buffer, error) {
	for _, o := range opts {
		o(&g.opts)
	}

	ref := strings.TrimPrefix(uri, fmt.Sprintf("%s://", registry.OCIScheme))
	if idx := strings.LastIndex(ref, ":"); idx > 0 {
		g.opts.version = ref[idx+1:]
		ref = ref[0:idx]
	}

	u, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid chart URL format: %s", ref)
	}

	u, err = g.resolveURI(ref, g.opts.version, u)
	if err != nil {
		return nil, err
	}

	pullOpts := []registry.PullOption{
		registry.PullOptWithChart(true),
		registry.PullOptIgnoreMissingProv(true),
	}

	result, err := g.client.Pull(u.String(), pullOpts...)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(result.Chart.Data), nil
}

func (g *ociGetter) resolveURI(ref, version string, u *url.URL) (*url.URL, error) {
	var tag string
	var err error

	// Evaluate whether an explicit version has been provided. Otherwise, determine version to use
	_, errSemVer := semver.NewVersion(version)
	if errSemVer == nil {
		tag = version
	} else {
		// Retrieve list of repository tags
		tags, err := g.client.Tags(strings.TrimPrefix(ref, fmt.Sprintf("%s://", registry.OCIScheme)))
		if err != nil {
			return nil, err
		}
		if len(tags) == 0 {
			return nil, errors.Errorf("Unable to locate any tags in provided repository: %s", ref)
		}

		// Determine if version provided
		// If empty, try to get the highest available tag
		// If exact version, try to find it
		// If semver constraint string, try to find a match
		tag, err = registry.GetTagMatchingVersionOrConstraint(tags, version)
		if err != nil {
			return nil, err
		}
	}

	u.Path = fmt.Sprintf("%s:%s", u.Path, tag)

	return u, err
}
