package pipeline

import (
	"strings"
	"text/template"
	"time"
)

// promptData is what a prompt template can use: {{.Text}}, {{.Mode}}, {{.Time}}.
type promptData struct {
	Text string
	Mode string
	Time string
}

// renderPrompt executes the prompt template. usesText reports whether the
// template placed {{.Text}} itself, so the caller doesn't attach it twice.
func renderPrompt(tmpl string, data promptData) (prompt string, usesText bool, err error) {
	t, err := template.New("prompt").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", false, err
	}
	return b.String(), strings.Contains(tmpl, ".Text"), nil
}

func now() string { return time.Now().Format("2006-01-02 15:04") }
