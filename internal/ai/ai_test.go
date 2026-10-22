package ai

import (
	"context"
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Miwm64/peekai/internal/config"
)

// fakeServer records the last chat request and replies with status/body.
func fakeServer(t *testing.T, status int, body string) (*httptest.Server, *http.Request, *chatRequest) {
	t.Helper()
	var gotReq http.Request
	var gotBody chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = *r
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &gotReq, &gotBody
}

func cfgFor(provider, baseURL string) config.AI {
	return config.AI{Provider: provider, BaseURL: baseURL, Model: "m1", Timeout: config.Duration(5 * time.Second)}
}

const okBody = `{"choices":[{"message":{"content":"  the answer \n"}}]}`

func TestAnalyzeText(t *testing.T) {
	srv, req, body := fakeServer(t, http.StatusOK, okBody)
	p, err := New(cfgFor("ollama", srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := p.Analyze(context.Background(), Request{Prompt: "Explain", Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "the answer" {
		t.Errorf("answer = %q", got)
	}
	if req.URL.Path != "/chat/completions" {
		t.Errorf("path = %s", req.URL.Path)
	}
	if h := req.Header.Get("Authorization"); h != "" {
		t.Errorf("ollama sent Authorization %q", h)
	}
	if body.Model != "m1" || len(body.Messages) != 1 {
		t.Fatalf("body = %+v", body)
	}
	if c, _ := body.Messages[0].Content.(string); c != "Explain\n\nScreen text:\nhello" {
		t.Errorf("content = %q", body.Messages[0].Content)
	}
}

func TestAnalyzeImageWithKey(t *testing.T) {
	t.Setenv("ROUTERAI_API_KEY", "secret")
	srv, req, body := fakeServer(t, http.StatusOK, okBody)
	p, err := New(cfgFor("routerai", srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if _, err := p.Analyze(context.Background(), Request{Prompt: "Explain", Image: img}); err != nil {
		t.Fatal(err)
	}
	if h := req.Header.Get("Authorization"); h != "Bearer secret" {
		t.Errorf("Authorization = %q", h)
	}
	parts, _ := body.Messages[0].Content.([]any)
	if len(parts) != 2 {
		t.Fatalf("content parts = %#v", body.Messages[0].Content)
	}
	imgPart, _ := parts[1].(map[string]any)
	url, _ := imgPart["image_url"].(map[string]any)["url"].(string)
	if imgPart["type"] != "image_url" || !strings.HasPrefix(url, "data:image/jpeg;base64,") {
		t.Errorf("image part = %#v", imgPart)
	}
}

func TestAnalyzeErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"openai style", http.StatusUnauthorized, `{"error":{"message":"bad key"}}`, "bad key"},
		{"ollama style", http.StatusNotFound, `{"error":"model not found"}`, "model not found"},
		{
			"ollama nested", http.StatusBadRequest,
			`{"error":{"message":"{\"error\":{\"code\":400,\"message\":\"inner reason\"}}","type":"invalid_request_error"}}`,
			"400 Bad Request: inner reason",
		},
		{"plain text", http.StatusBadGateway, `upstream down`, "upstream down"},
		{"no choices", http.StatusOK, `{"choices":[]}`, "empty response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _, _ := fakeServer(t, tt.status, tt.body)
			p, err := New(cfgFor("openai_compat", srv.URL))
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.Analyze(context.Background(), Request{Prompt: "x"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Setenv("XAI_API_KEY", "")
	tests := []struct {
		name string
		cfg  config.AI
		want string
	}{
		{"unknown provider", cfgFor("gpt", ""), `unknown "gpt"`},
		{"generic needs url", cfgFor("openai_compat", ""), "ai.base_url"},
		{"grok needs key", cfgFor("grok", ""), "XAI_API_KEY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestPresetDefaults(t *testing.T) {
	t.Setenv("XAI_API_KEY", "k")
	p, err := New(cfgFor("grok", ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.(*openAI).baseURL; got != "https://api.x.ai/v1" {
		t.Errorf("grok baseURL = %s", got)
	}

	t.Setenv("OPENROUTER_API_KEY", "k")
	p, err = New(cfgFor("openrouter", ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.(*openAI).baseURL; got != "https://openrouter.ai/api/v1" {
		t.Errorf("openrouter baseURL = %s", got)
	}

	t.Setenv("REQUESTY_API_KEY", "k")
	p, err = New(cfgFor("requesty", ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.(*openAI).baseURL; got != "https://router.requesty.ai/v1" {
		t.Errorf("requesty baseURL = %s", got)
	}
}

func TestCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	p, err := New(cfgFor("ollama", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Analyze(ctx, Request{Prompt: "x"}); err == nil {
		t.Error("want error on cancelled context")
	}
}

func TestNoVisionHint(t *testing.T) {
	body := `{"error":{"message":"Multimodal data provided, but model does not support multimodal requests."}}`
	srv, _, _ := fakeServer(t, http.StatusBadRequest, body)
	p, err := New(cfgFor("ollama", srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Analyze(context.Background(), Request{Prompt: "x", Image: image.NewRGBA(image.Rect(0, 0, 2, 2))})
	if err == nil || !strings.Contains(err.Error(), `model "m1" can't read images`) {
		t.Errorf("err = %v, want vision hint", err)
	}

	_, err = p.Analyze(context.Background(), Request{Prompt: "x", Text: "words"})
	if err == nil || strings.Contains(err.Error(), "can't read images") {
		t.Errorf("text request got vision hint: %v", err)
	}
}

func TestHistoryIsSentAsMessages(t *testing.T) {
	srv, _, body := fakeServer(t, http.StatusOK, okBody)
	p, err := New(cfgFor("ollama", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Prompt: "And now?", History: []Turn{{User: "Explain", Assistant: "It is a chart."}}}
	if _, err := p.Analyze(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	var roles []string
	for _, m := range body.Messages {
		roles = append(roles, m.Role)
	}
	if strings.Join(roles, ",") != "user,assistant,user" {
		t.Fatalf("roles = %v", roles)
	}
	if body.Messages[1].Content != "It is a chart." || body.Messages[2].Content != "And now?" {
		t.Errorf("messages = %+v", body.Messages)
	}
}

func TestKeyIsTrimmedAndAuthErrorNamesEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "  'sk-or-123'\n")
	srv, req, _ := fakeServer(t, http.StatusUnauthorized, `{"error":{"message":"Missing Authentication header","code":401}}`)
	p, err := New(cfgFor("openrouter", srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Analyze(context.Background(), Request{Prompt: "x"})
	if h := req.Header.Get("Authorization"); h != "Bearer sk-or-123" {
		t.Errorf("Authorization = %q", h)
	}
	if err == nil || !strings.Contains(err.Error(), "check the key in OPENROUTER_API_KEY") {
		t.Errorf("err = %v", err)
	}
}
