package certificates

import (
	"os"
	"path/filepath"

	"github.com/krateoplatformops/core-provider/internal/tools/pluralizer"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
)

type FuncOption func(o *options)

type options struct {
	path       string
	pluralizer pluralizer.PluralizerInterface
	log        func(msg string, keysAndValues ...any)
}

func defaultOptions() options {
	return options{
		path:       filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs"),
		pluralizer: pluralizer.New(false),
		log:        logging.NewNopLogger().Debug, // No-op logger
	}
}

func WithCertPath(path string) func(*options) {
	return func(o *options) {
		o.path = path
	}
}

func WithPluralizer(p pluralizer.PluralizerInterface) func(*options) {
	return func(o *options) {
		o.pluralizer = p
	}
}

func WithLogger(log func(msg string, keysAndValues ...any)) func(*options) {
	return func(o *options) {
		o.log = log
	}
}
