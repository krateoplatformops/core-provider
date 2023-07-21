package tgz

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	chartSizeLimit = 1 << 20 // 1MB
)

func Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Krateo")

	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 20 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 20 * time.Second,
	}

	client := &http.Client{
		Timeout:   time.Second * 40,
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	out := &bytes.Buffer{}
	src := io.LimitReader(resp.Body, chartSizeLimit)
	_, err = io.Copy(out, src)

	return out.Bytes(), err
}
