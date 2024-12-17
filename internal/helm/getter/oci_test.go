package getter

import (
	"testing"
)

func TestOCI(t *testing.T) {
	const (
		uri     = "oci://registry-1.docker.io/bitnamicharts/redis"
		version = "18.0.1"
	)

	g, err := newOCIGetter(".")
	if err != nil {
		t.Fatal(err)
	}

	dat, _, err := g.Get(GetOptions{
		URI:     uri,
		Version: version,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(dat) == 0 {
		t.Fatal("expected tgz archive, got zero bytes!")
	}
}
