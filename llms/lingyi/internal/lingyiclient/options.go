package lingyiclient

// Option is an option for the Lingyi client.
type Option func(*Client)

func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithAPIKey(apikey string) Option {
	return func(c *Client) {
		c.apikey = apikey
	}
}

func WithHTTPClient(httpclient Doer) Option {
	return func(c *Client) {
		c.httpClient = httpclient
	}
}
