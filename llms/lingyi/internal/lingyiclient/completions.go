package lingyiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"` // TODO: Content can be array
}

// CompletionRequest is a request to complete a completion.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream"`

	// StreamingFunc is a function to be called for each chunk of a streaming response.
	// Return an error to stop streaming early.
	StreamingFunc func(ctx context.Context, chunk []byte) error `json:"-"`
}

type CompletionResponseChoice struct {
	FinishReason string  `json:"finish_reason,omitempty"`
	Index        float64 `json:"index,omitempty"`
	Message      Message `json:"message"`
}

type CompletionResponse struct {
	ID      string                     `json:"id,omitempty"`
	Created float64                    `json:"created,omitempty"`
	Choices []CompletionResponseChoice `json:"choices,omitempty"`
	Model   string                     `json:"model,omitempty"`
	Object  string                     `json:"object,omitempty"`
	Usage   struct {
		CompletionTokens float64 `json:"completion_tokens,omitempty"`
		PromptTokens     float64 `json:"prompt_tokens,omitempty"`
		TotalTokens      float64 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

// StreamedChatResponsePayload is a chunk from the stream.
type StreamedChatResponsePayload struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string  `json:"finish_reason,omitempty"`
		Index        float64 `json:"index,omitempty"`
	} `json:"choices,omitempty"`
	Content string  `json:"content,omitempty"`
	Created float64 `json:"created,omitempty"`
	ID      string  `json:"id,omitempty"`
	LastOne bool    `json:"lastOne,omitempty"`
	Model   string  `json:"model,omitempty"`
	Object  string  `json:"object,omitempty"`
	Usage   struct {
		CompletionTokens float64 `json:"completion_tokens,omitempty"`
		PromptTokens     float64 `json:"prompt_tokens,omitempty"`
		TotalTokens      float64 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
	Error error `json:"-"`
}

type errorMessage struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   any    `json:"param,omitempty"`
		Type    string `json:"type,omitempty"`
	} `json:"error"`
}

func (c *Client) setCompletionDefaults(payload *CompletionRequest) {
	// Set defaults
	if payload.MaxTokens == 0 {
		payload.MaxTokens = 256
	}

	switch {
	// Prefer the model specified in the payload.
	case payload.Model != "":

	// If no model is set in the payload, take the one specified in the client.
	case c.model != "":
		payload.Model = c.model
	// Fallback: use the default model
	default:
		payload.Model = defaultCompletionModel
	}
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
}

func (c *Client) createCompletion(ctx context.Context, payload *CompletionRequest) (*CompletionResponse, error) {
	c.setCompletionDefaults(payload)
	if payload.StreamingFunc != nil {
		payload.Stream = true
	}
	// Build request payload

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Build request
	body := bytes.NewReader(payloadBytes)
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/chat/completions", c.baseURL), body)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	// Send request
	r, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("API returned unexpected status code: %d", r.StatusCode)

		// No need to check the error here: if it fails, we'll just return the
		// status code.
		var errResp errorMessage
		if err := json.NewDecoder(r.Body).Decode(&errResp); err != nil {
			return nil, errors.New(msg) // nolint:goerr113
		}

		return nil, fmt.Errorf("%s: %s", msg, errResp.Error.Message) // nolint:goerr113
	}
	if payload.StreamingFunc != nil {
		return parseStreamingChatResponse(ctx, r, payload)
	}
	// Parse response
	var response CompletionResponse
	return &response, json.NewDecoder(r.Body).Decode(&response)
}

func parseStreamingChatResponse(ctx context.Context, r *http.Response, payload *CompletionRequest) (*CompletionResponse, error) { //nolint:cyclop,lll
	scanner := bufio.NewScanner(r.Body)
	responseChan := make(chan StreamedChatResponsePayload)
	go func() {
		defer close(responseChan)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			data := strings.TrimPrefix(line, "data:") // here use `data:` instead of `data: ` for compatibility
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				return
			}
			var streamPayload StreamedChatResponsePayload
			err := json.NewDecoder(bytes.NewReader([]byte(data))).Decode(&streamPayload)
			if err != nil {
				streamPayload.Error = fmt.Errorf("error decoding streaming response: %w", err)
				responseChan <- streamPayload
				return
			}
			responseChan <- streamPayload
		}
		if err := scanner.Err(); err != nil {
			responseChan <- StreamedChatResponsePayload{Error: fmt.Errorf("error reading streaming response: %w", err)}
			return
		}
	}()
	// Combine response
	return combineStreamingChatResponse(ctx, payload, responseChan)
}

func combineStreamingChatResponse(ctx context.Context, payload *CompletionRequest, responseChan chan StreamedChatResponsePayload) (*CompletionResponse, error) {
	response := CompletionResponse{
		Choices: []CompletionResponseChoice{
			{},
		},
	}

	for streamResponse := range responseChan {
		if streamResponse.Error != nil {
			return nil, streamResponse.Error
		}

		if len(streamResponse.Choices) == 0 {
			continue
		}
		choice := streamResponse.Choices[0]
		chunk := []byte(choice.Delta.Content)
		response.Choices[0].Message.Content += choice.Delta.Content
		response.Choices[0].FinishReason = choice.FinishReason

		if payload.StreamingFunc != nil {
			err := payload.StreamingFunc(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("streaming func returned an error: %w", err)
			}
		}
	}
	return &response, nil
}
