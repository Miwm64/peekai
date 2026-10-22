package ai

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Miwm64/peekai/internal/config"
)

// New builds a client for one configured model. Every provider speaks the
// OpenAI chat/completions API; its URL and key variable come from the
// config, so adding a provider needs no code.
func New(m config.ResolvedModel) (Provider, error) {
	var key string
	if env := m.Provider.KeyEnv; env != "" {
		// Copy-pasted keys often carry spaces or quotes; providers then
		// reject them with confusing messages.
		key = strings.Trim(strings.TrimSpace(os.Getenv(env)), `"'`)
		if key == "" {
			return nil, fmt.Errorf("%s: set the API key in the %s environment variable", m.Provider.Name, env)
		}
	}
	return &openAI{
		name:            m.Provider.Name,
		baseURL:         strings.TrimRight(m.Provider.URL, "/"),
		model:           m.Model.Model,
		reasoningEffort: m.ReasoningEffort,
		apiKey:          key,
		keyEnv:          m.Provider.KeyEnv,
		client:          &http.Client{Timeout: time.Duration(m.Timeout)},
	}, nil
}
