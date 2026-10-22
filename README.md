# peekai

Press a shortcut, and an AI looks at your screen. Press another, and the answer shows
up in a popup or a running line, on top of whatever you're working in. Fullscreen apps
and other displays are included.

- **Shortcuts only.** Capturing opens no window. Windows open and close only on their own
  shortcuts, without taking focus from your app.
- **Image or OCR.** Send the screenshot to a vision model, or send only the screen text
  (Tesseract or a vision model reads it).
- **Any OpenAI-compatible provider.** Ollama (local), Gemini, OpenRouter, RouterAI,
  Requesty, Grok or your own server. Define providers and models in `config.json` and
  switch models with a shortcut or the tray.
- **Conversation context.** Optionally follow up on earlier answers.
- **Invisible to screenshots.** The overlay never captures itself, and screen sharing
  doesn't show it on macOS and Windows.
- macOS, Windows and Linux (X11).

## Install

Download the latest release for your OS from
[Releases](https://github.com/Miwm64/peekai/releases):

| OS | File | Run |
|---|---|---|
| macOS (Apple Silicon + Intel) | `peekai-…-macos-universal.zip` | unzip, move `peekai.app` anywhere, open it |
| Windows | `peekai-…-windows-amd64.zip` | unzip, run `peekai.exe` |
| Linux | `peekai-…-linux-amd64.tar.gz` | unpack, run `./peekai` |

The release builds aren't signed by Apple or Microsoft, so the OS warns you on first start:
- **macOS:** right-click `peekai.app` and choose **Open**, then **Open** again. Or run
  `xattr -dr com.apple.quarantine peekai.app`.
- **Windows:** in SmartScreen, click **More info**, then **Run anyway**.

### Build from source

You need Go (the version in `go.mod`) and a C compiler on macOS and Linux. On Linux you
also need `libx11-dev libxtst-dev libgl1-mesa-dev libegl1-mesa-dev`.

```bash
make build      # bin/peekai
make app        # macOS: bin/peekai.app (universal)
go run ./cmd/peekai
```

### Permissions

- **macOS:** the first run asks for **Screen Recording** (to take screenshots) and
  **Accessibility** (for global shortcuts). Grant them in System Settings → Privacy &
  Security, to `peekai.app`, or to your terminal if you run it from there. Then restart
  peekai.
- **Linux:** shortcuts and window placement need X11. On Wayland desktops peekai runs
  through XWayland; native Wayland isn't supported yet.
- **Windows:** nothing to grant.

## Usage

Start peekai. It runs in the background with a tray icon. Then use it from any app:

| Shortcut | Action |
|---|---|
| **Ctrl+Shift+S** | take a screenshot of the display under the mouse and ask the AI |
| **Ctrl+Shift+1** | show / hide the popup |
| **Ctrl+Shift+2** | show / hide the running line |
| Ctrl+Shift+H | hide all windows (a running request keeps going) |
| Ctrl+Shift+N | next prompt |
| Ctrl+Shift+M | switch image / OCR mode |
| Ctrl+Shift+P | next model (in `ai.models` order) |
| Ctrl+Shift+C | copy the last answer |
| Ctrl+Shift+X | clear: forget the answer and the AI context, cancel a running request |
| Ctrl+Shift+R | reload `config.json` |
| Ctrl+Shift+Q | quit |

The popup's title line shows where things stand:
`● thinking… · explain · image · qwen-local · free`, then
`✓ answer · … · 14:02`. After you switch prompt, mode or model, the answer stays
visible and is marked **previous answer**.

The tray menu shows the same status, and has a **Models** submenu (pick any model
directly), show/hide, **Reload config** and **Quit**.

Switching mode, prompt or model only affects the running app; `config.json` is never
changed. The next start begins with the defaults from the file again.

### Command line

```bash
peekai                     # run in the background with shortcuts (normal use)
peekai -once               # analyze the screen once, print the answer, exit
peekai -once -mode ocr -prompt translate -model grok -out sent.png
peekai -console            # shortcuts, but print results in the terminal
peekai -config path/to/config.json
peekai -version
```

## Configuration

On first start peekai writes `config.json` **next to the program**: next to
`peekai.app` on macOS, or in the current folder under `go run`. Edit it and press
**Ctrl+Shift+R** to apply. A broken config is reported, and the previous one keeps running.

```json
{
  "version": 2,
  "capture": { "mode": "image", "display": "mouse", "max_image_px": 1600 },
  "ocr": { "engine": "tesseract_cli", "lang": "eng+rus", "path": "" },
  "ai": {
    "default_model": "qwen-local",
    "auto_clear": false,
    "timeout": "2m0s",
    "providers": [
      { "name": "ollama",   "url": "http://localhost:11434/v1" },
      { "name": "grok",     "url": "https://api.x.ai/v1", "key_env": "XAI_API_KEY" },
      { "name": "routerai", "url": "https://routerai.ru/api/v1", "key_env": "ROUTERAI_API_KEY" }
    ],
    "models": [
      { "name": "qwen-local", "provider": "ollama", "model": "qwen2.5vl:7b",
        "vision": true, "free": true, "description": "local, private, slower" },
      { "name": "grok", "provider": "grok", "model": "grok-4.7",
        "vision": true, "free": false, "description": "xAI, prepaid credits" },
      { "name": "deepseek-flash", "provider": "routerai", "model": "deepseek/deepseek-v4.1-flash",
        "vision": true, "free": false, "description": "cheap, via RouterAI" }
    ]
  },
  "prompts": {
    "default": "explain",
    "list": {
      "explain": "Explain what is on this screen concisely.",
      "translate": "Translate all text to English.",
      "answer": "Answer the question shown on screen. Be brief."
    }
  },
  "hotkeys": { "capture": "Ctrl+Shift+S", "toggle_popup": "Ctrl+Shift+1", "…": "…" },
  "ui": {
    "theme": "dark",
    "themes": [],
    "popup": { "width": 520, "max_height": 600, "opacity": 0.95, "autohide": "0s" },
    "ticker": { "speed": 120, "height": 44, "opacity": 0.85, "position": "bottom", "start": "right" },
    "status": { "enabled": false }
  }
}
```

| Key | Meaning |
|---|---|
| `version` | config format; must be `2` |
| `capture.mode` | mode at startup: `image` sends the screenshot, `ocr` only the screen text (Ctrl+Shift+M switches) |
| `capture.display` | `mouse` (the display under the mouse), `main`, `all`, or a display number like `"1"` |
| `capture.max_image_px` | longest side of the screenshot sent to the AI (`0` = full size) |
| `ocr.engine` | `tesseract_cli` (local; needs [Tesseract](https://tesseract-ocr.github.io/tessdoc/Installation.html)) or `llm` (the current model reads the text; needs a vision model) |
| `ocr.lang` | Tesseract languages joined with `+` (default `eng+rus`); `tesseract --list-langs` shows what's installed. Fewer languages = faster and more accurate |
| `ai.default_model` | name of the model to start with |
| `ai.auto_clear` | `true`: every capture starts a fresh AI conversation. `false`: earlier prompts and answers (text only) are sent as context until Ctrl+Shift+X |
| `ai.timeout` | per request, e.g. `"90s"` |
| `ai.providers` | API endpoints (see below) |
| `ai.models` | models to use; Ctrl+Shift+P cycles them in this order |
| `prompts.default` | prompt to start with (Ctrl+Shift+N cycles) |
| `prompts.list` | named prompts; Go templates with `{{.Text}}` (OCR text), `{{.Mode}}`, `{{.Time}}` |
| `hotkeys.*` | `Ctrl`, `Shift`, `Alt`/`Option`, `Cmd`/`Win`/`Super` + a key (`A`–`Z`, `0`–`9`, `F1`–`F12`, `Space`, `Enter`, `Esc`, `Tab`, `Delete`, arrows) |
| `ui.theme` | look at startup; built in: `dark` (more themes are coming) |
| `ui.ticker.start` | `right`: enters at the right edge; `left`: starts visible at the left |
| `ui.popup.autohide` | hide the popup after this long, e.g. `"20s"` (`"0s"` = never) |
| `ui.status.enabled` | small top-right pill with "thinking…" and notices |

Unknown keys are rejected, so typos get caught. Lists you write (providers, models,
prompts) replace the defaults completely.

## Providers and models

**A provider** is an OpenAI-compatible API endpoint:

| Field | |
|---|---|
| `name` | your name for it, used by models |
| `url` | base URL ending before `/chat/completions`, e.g. `https://api.x.ai/v1` |
| `key_env` | environment variable holding the API key; leave it out for keyless servers (Ollama, LM Studio) |

**A model** is what you switch between:

| Field | |
|---|---|
| `name` | shown in the title line and tray |
| `provider` | a provider's `name` |
| `model` | the provider's model ID |
| `vision` | `true` if it reads images. In image mode, a model with `vision: false` is refused before anything is sent; use OCR mode for it |
| `free` | shown as `free` in the title and tray |
| `description` | shown in the tray's Models menu |
| `reasoning_effort` | optional: `none`, `low`, `medium`, `high`. `none` makes "thinking" models (e.g. Gemini Flash) answer much faster |

API keys come from environment variables and never go in the config file. A model whose
provider's key isn't set is skipped when you switch to it.

### Ready-to-paste providers

```json
{ "name": "gemini",     "url": "https://generativelanguage.googleapis.com/v1beta/openai", "key_env": "GEMINI_API_KEY" },
{ "name": "openrouter", "url": "https://openrouter.ai/api/v1", "key_env": "OPENROUTER_API_KEY" },
{ "name": "requesty",   "url": "https://router.requesty.ai/v1", "key_env": "REQUESTY_API_KEY" },
{ "name": "lmstudio",   "url": "http://localhost:1234/v1" },
{ "name": "openai",     "url": "https://api.openai.com/v1", "key_env": "OPENAI_API_KEY" }
```

And models for them:

```json
{ "name": "gemini-flash", "provider": "gemini", "model": "gemini-3.8-flash",
  "vision": true, "free": true, "description": "free tier, daily limits", "reasoning_effort": "none" },
{ "name": "or-free", "provider": "openrouter", "model": "openrouter/free",
  "vision": true, "free": true, "description": "rotating free models" },
{ "name": "llama-local", "provider": "ollama", "model": "llama3.1",
  "vision": false, "free": true, "description": "text only: use OCR mode" }
```

### Getting free keys

- **Ollama:** install from ollama.com, then run `ollama pull qwen2.5vl:7b`. No key.
- **Gemini:** aistudio.google.com → **Get API key**. No card needed. On the free tier,
  Google may use your inputs to improve its models.
- **OpenRouter:** openrouter.ai → **Keys** → **Create key**. Use the model `openrouter/free`,
  which picks a free model that can read images.

```bash
export GEMINI_API_KEY=...        # add to ~/.zshrc or ~/.bashrc to keep it
```

## Migrating to version 2

peekai refuses a version 1 config with *"old config format (version 1)"*. Either delete
`config.json` (peekai writes new defaults) or update it:

| Version 1 | Version 2 |
|---|---|
| `mode` | `capture.mode` |
| `auto_clear` | `ai.auto_clear` |
| `capture.display: "active"` | `capture.display: "mouse"` |
| `ai.max_image_px` | `capture.max_image_px` |
| `ai.provider` + `ai.base_url` + `ai.api_key_env` | an entry in `ai.providers` (`name`, `url`, `key_env`) |
| `ai.model` (+ `ai.alternatives`) | entries in `ai.models` (`name`, `provider`, `model`, `vision`, …) and `ai.default_model` |
| `prompts.active` | `prompts.default` |
| `hotkeys.cycle_provider` | `hotkeys.cycle_model` |
| — | `"version": 2` at the top |

Before:
```json
{ "mode": "ocr", "ai": { "provider": "ollama", "model": "qwen2.5vl:7b",
  "alternatives": [ { "provider": "grok", "model": "grok-4.7" } ] } }
```
After:
```json
{ "version": 2,
  "capture": { "mode": "ocr" },
  "ai": {
    "default_model": "qwen",
    "providers": [ { "name": "ollama", "url": "http://localhost:11434/v1" },
                   { "name": "grok", "url": "https://api.x.ai/v1", "key_env": "XAI_API_KEY" } ],
    "models": [ { "name": "qwen", "provider": "ollama", "model": "qwen2.5vl:7b", "vision": true, "free": true },
                { "name": "grok", "provider": "grok", "model": "grok-4.7", "vision": true } ] } }
```

## Troubleshooting

| Problem | Fix |
|---|---|
| `failed to register … Accessibility` | macOS: allow peekai (or your terminal) in Privacy & Security → Accessibility, then restart it |
| `cannot capture display` | macOS: allow Screen Recording, then restart |
| `model "…" can't read images` | the model is marked `vision: false` (or the provider says so): switch to OCR mode (Ctrl+Shift+M) or a vision model (Ctrl+Shift+P) |
| `old config format (version 1)` | see [Migrating to version 2](#migrating-to-version-2) |
| `401 … (check the key in X_API_KEY)` | the key in that variable is wrong. OpenRouter says "Missing Authentication header" even for wrong keys |
| `context deadline exceeded` | the model is slow (local models while loading, "thinking" models): raise `ai.timeout`, or set `"reasoning_effort": "none"` on the model |
| `unknown field "…"` | that config key doesn't exist (any more); remove it, or delete `config.json` to get fresh defaults |
| startup errors are invisible in `peekai.app` | start it from a terminal (`peekai.app/Contents/MacOS/peekai`) to see them |

## Development

```bash
make test       # go test -race ./...
make lint       # golangci-lint v2.15.0, as in CI
```

- **Layout:** `internal/` packages each have one interface and a factory:
  - `capture`, `ocr`, `ai`: the stages
  - `pipeline`: runs them
  - `app`: the controller, which turns shortcuts into behaviour
  - `hotkey`: global shortcuts
  - `ui/overlay` (go-gui windows) plus `ui/ontop` (per-OS always-on-top, placement and
    screenshot exclusion)
  - `config`
  - `docs/ROADMAP.md` has the plan.
- **Contributing:** one branch per feature from `master`, with
  [Conventional Commits](https://www.conventionalcommits.org/). CI runs lint, tests and
  builds on all three OSes. Pushing a `v*` tag publishes a release.
