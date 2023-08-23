package getter

import (
	"bytes"
)

// options are generic parameters to be provided to the getter during instantiation.
//
// Getters may or may not ignore these parameters as they are passed in.
type options struct {
	insecureSkipVerifyTLS bool
	plainHTTP             bool
	username              string
	password              string
	passCredentialsAll    bool
	version               string
}

// Option allows specifying various settings configurable by the user for overriding the defaults
// used when performing Get operations with the Getter.
type Option func(*options)

// WithBasicAuth sets the request's Authorization header to use the provided credentials
func WithBasicAuth(username, password string) Option {
	return func(opts *options) {
		opts.username = username
		opts.password = password
	}
}

func WithPassCredentialsAll(pass bool) Option {
	return func(opts *options) {
		opts.passCredentialsAll = pass
	}
}

// WithInsecureSkipVerifyTLS determines if a TLS Certificate will be checked
func WithInsecureSkipVerifyTLS(insecureSkipVerifyTLS bool) Option {
	return func(opts *options) {
		opts.insecureSkipVerifyTLS = insecureSkipVerifyTLS
	}
}

func WithPlainHTTP(plainHTTP bool) Option {
	return func(opts *options) {
		opts.plainHTTP = plainHTTP
	}
}

func WithTagName(tagname string) Option {
	return func(opts *options) {
		opts.version = tagname
	}
}

// Getter is an interface to support GET to the specified URI.
type Getter interface {
	// Get file content by url string
	Get(uri string, opts ...Option) (*bytes.Buffer, error)
}
