package ai

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Miwm64/peekai/internal/config"
)

// preset holds a provider's defaults. Adding a provider that speaks the
// OpenAI chat/completions API is one entry here.
type preset struct {
	baseURL string
	keyEnv  string // env var holding the API key; "" = no key needed
}

var presets = map[string]preset{
	"ollama":        {baseURL: "http://localhost:11434/v1"},
	"routerai":      {baseURL: "https://routerai.ru/api/v1", keyEnv: "ROUTERAI_API_KEY"},
	"grok":          {baseURL: "https://api.x.ai/v1", keyEnv: "XAI_API_KEY"},
	"openrouter":    {baseURL: "https://openrouter.ai/api/v1", keyEnv: "OPENROUTER_API_KEY"},
	"requesty":      {baseURL: "https://router.requesty.ai/v1", keyEnv: "REQUESTY_API_KEY"},
	"openai_compat": {}, // generic: base_url from config, key optional
}

// Providers lists the supported provider names.
func Providers() []string {
	return slices.Sorted(maps.Keys(presets))
}

// New builds the Provider named by cfg.Provider, applying config overrides
// on top of the preset.
func New(cfg config.AI) (Provider, error) {
	p, ok := presets[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("ai.provider: unknown %q (want one of %s)", cfg.Provider, strings.Join(Providers(), ", "))
	}

	baseURL := cmp.Or(cfg.BaseURL, p.baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("ai.base_url: required for provider %q", cfg.Provider)
	}

	keyEnv := cmp.Or(cfg.APIKeyEnv, p.keyEnv)
	var key string
	if keyEnv != "" {
		key = os.Getenv(keyEnv)
		// Presets with a key env need it; a generic server may run keyless.
		if key == "" && p.keyEnv != "" {
			return nil, fmt.Errorf("%s: set the API key in the %s environment variable", cfg.Provider, keyEnv)
		}
	}

	return &openAI{
		name:    cfg.Provider,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   cfg.Model,
		apiKey:  key,
		client:  &http.Client{Timeout: time.Duration(cfg.Timeout)},
	}, nil
}
