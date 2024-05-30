package lingyiclient

import (
	"context"
	"errors"
	"net/http"
)

const (
	defaultBaseURL         = "https://api.lingyiwanwu.com/v1"
	defaultCompletionModel = "yi-large"
)

// ErrEmptyResponse is returned when the OpenAI API returns an empty response.
var ErrEmptyResponse = errors.New("empty response")

type Client struct {
	apikey     string
	model      string
	baseURL    string
	httpClient Doer
}

// Doer performs a HTTP request.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// New returns a new OpenAI client.
func New(opts ...Option) (*Client, error) {
	c := &Client{}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Completion is a completion.
type Completion struct {
	CompletionTokens float64 `json:"completion_tokens,omitempty"`
	PromptTokens     float64 `json:"prompt_tokens,omitempty"`
	TotalTokens      float64 `json:"total_tokens,omitempty"`
	Content          string  `json:"content"`
}

// CreateCompletion creates a completion.
func (c *Client) CreateCompletion(ctx context.Context, r *CompletionRequest) (*Completion, error) {
	resp, err := c.createCompletion(ctx, r)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, ErrEmptyResponse
	}
	return &Completion{
		Content:          resp.Choices[0].Message.Content,
		CompletionTokens: resp.Usage.CompletionTokens,
		PromptTokens:     resp.Usage.PromptTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apikey)
}
