package getter

import (
	"testing"
)

func TestTGZ(t *testing.T) {
	const (
		uri = "https://github.com/krateoplatformops/krateo-v2-template-fireworksapp/releases/download/0.1.0/fireworks-app-0.1.0.tgz"
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
