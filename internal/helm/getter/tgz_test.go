package getter

import (
	"testing"
)

func TestGTZ(t *testing.T) {
	const (
		uri = "https://github.com/krateoplatformops/krateo-v2-template-fireworksapp/archive/refs/tags/0.0.1.tar.gz"
	)

	if !isTGZ(uri) {
		t.Fatal("expected Tar Gz URI!")
	}

	dat, _, err := Get(GetOptions{
		URI: uri,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(dat) == 0 {
		t.Fatal("expected tgz archive, got zero bytes!")
	}
}
