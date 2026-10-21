package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openAI talks to any server implementing POST /chat/completions.
type openAI struct {
	name    string // provider name, used in errors
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string, or []contentPart with an image
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (p *openAI) Analyze(ctx context.Context, req Request) (string, error) {
	msg := chatMessage{Role: "user", Content: userText(req)}
	if req.Image != nil {
		url, err := dataURL(req.Image)
		if err != nil {
			return "", fmt.Errorf("encode image: %w", err)
		}
		msg.Content = []contentPart{
			{Type: "text", Text: userText(req)},
			{Type: "image_url", ImageURL: &imageURL{URL: url}},
		}
	}

	body, err := json.Marshal(chatRequest{Model: p.model, Messages: []chatMessage{msg}})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%s: %w", p.name, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("%s: read response: %w", p.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s: %s", p.name, resp.Status, apiError(data))
	}

	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("%s: decode response: %w", p.name, err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: empty response", p.name)
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// apiError extracts a readable message from an error response body.
// Servers use either {"error": {"message": "..."}} or {"error": "..."}.
func apiError(body []byte) string {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && len(e.Error) > 0 {
		var obj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Error, &obj) == nil && obj.Message != "" {
			return obj.Message
		}
		var s string
		if json.Unmarshal(e.Error, &s) == nil && s != "" {
			return s
		}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if msg == "" {
		return "no details"
	}
	return msg
}
