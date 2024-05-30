package lingyi

import (
	"net/http"

	"github.com/tmc/langchaingo/llms/lingyi/internal/lingyiclient"
)

type options struct {
	lingyiOptions []lingyiclient.Option
}

type Option func(*options)

// WithAPIKey Set the model to use.
func WithAPIKey(apikey string) Option {
	return func(opts *options) {
		opts.lingyiOptions = append(opts.lingyiOptions, lingyiclient.WithAPIKey(apikey))
	}
}

// WithModel Set the model to use.
func WithModel(model string) Option {
	return func(opts *options) {
		opts.lingyiOptions = append(opts.lingyiOptions, lingyiclient.WithModel(model))
	}
}

// WithBaseURL Set the URL of the lingyi instance to use.
func WithBaseURL(rawURL string) Option {
	return func(opts *options) {
		opts.lingyiOptions = append(opts.lingyiOptions, lingyiclient.WithBaseURL(rawURL))
	}
}

// WithHTTPClient Set custom http client.
func WithHTTPClient(client *http.Client) Option {
	return func(opts *options) {
		opts.lingyiOptions = append(opts.lingyiOptions, lingyiclient.WithHttpClient(client))
	}
}
