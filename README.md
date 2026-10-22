# peekai

Press a shortcut, and an AI looks at your screen. Press another, and the answer shows
up in a popup or a running line, on top of whatever you're working in. Fullscreen apps
and other displays are included.

- **Shortcuts only.** Capturing opens no window. Windows open and close only on their own
  shortcuts, without taking focus from your app.
- **Image or OCR.** Send the screenshot to a vision model, or send only the screen text
  (Tesseract or a vision model reads it).
- **Any OpenAI-compatible provider.** Ollama (local), Gemini, OpenRouter, RouterAI,
  Requesty, Grok or your own server. Switch between them with a shortcut.
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
| Ctrl+Shift+P | next provider (`ai` → `ai.alternatives`) |
| Ctrl+Shift+C | copy the last answer |
| Ctrl+Shift+X | clear: forget the answer and the AI context, cancel a running request |
| Ctrl+Shift+R | reload `config.json` |
| Ctrl+Shift+Q | quit |

The popup's title line shows where things stand:
`● thinking… · explain · image · ollama/qwen2.5vl:7b`, then
`✓ answer · … · 14:02`. After you switch prompt, mode or provider, the answer stays
visible and is marked **previous answer**.

The tray menu shows the same status, and has show/hide, **Reload config** and **Quit**.

### Command line

```bash
peekai                     # run in the background with shortcuts (normal use)
peekai -once               # analyze the screen once, print the answer, exit
peekai -once -mode ocr -prompt translate -out sent.png
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
  "mode": "image",
  "auto_clear": true,
  "hotkeys": { "capture": "Ctrl+Shift+S", "toggle_popup": "Ctrl+Shift+1", "…": "…" },
  "capture": { "display": "active" },
  "ocr": { "engine": "tesseract_cli", "lang": "eng", "path": "" },
  "ai": {
    "provider": "ollama",
    "model": "qwen2.5vl:7b",
    "base_url": "",
    "api_key_env": "",
    "timeout": "2m0s",
    "max_image_px": 1600,
    "alternatives": [
      { "provider": "gemini", "model": "gemini-3.8-flash" },
      { "provider": "openrouter", "model": "openrouter/free" }
    ]
  },
  "prompts": {
    "active": "explain",
    "list": {
      "explain": "Explain what is on this screen concisely.",
      "translate": "Translate all text to English.",
      "answer": "Answer the question shown on screen. Be brief."
    }
  },
  "ui": {
    "ticker": { "speed": 120, "height": 44, "opacity": 0.85, "position": "bottom", "start": "right" },
    "popup": { "width": 520, "max_height": 600, "opacity": 0.95, "autohide": "0s" },
    "status": { "enabled": false }
  }
}
```

| Key | Meaning |
|---|---|
| `mode` | `image` sends the screenshot; `ocr` sends only the screen text |
| `auto_clear` | `true`: every capture starts a fresh AI conversation. `false`: earlier prompts and answers (text only) are sent as context until Ctrl+Shift+X |
| `hotkeys.*` | `Ctrl`, `Shift`, `Alt`/`Option`, `Cmd`/`Win`/`Super` + a key (`A`–`Z`, `0`–`9`, `F1`–`F12`, `Space`, `Enter`, `Esc`, `Tab`, `Delete`, arrows) |
| `capture.display` | `active` (under the mouse), `all`, or a display number like `"1"` |
| `ocr.engine` | `tesseract_cli` (needs [Tesseract](https://tesseract-ocr.github.io/tessdoc/Installation.html)) or `llm` (the AI model reads the text; needs a vision model) |
| `ocr.lang` | Tesseract languages, e.g. `eng`, `rus`, `eng+rus` |
| `ai.provider` | a preset (see below) or `openai_compat` with your own `base_url` |
| `ai.base_url`, `ai.api_key_env` | override the preset's address / key variable; empty = preset default |
| `ai.timeout` | per request, e.g. `"90s"` |
| `ai.max_image_px` | longest side of the screenshot sent to the AI (`0` = full size) |
| `ai.alternatives` | more `{provider, model}` pairs for Ctrl+Shift+P |
| `prompts.list` | named prompts; Go templates with `{{.Text}}` (OCR text), `{{.Mode}}`, `{{.Time}}` |
| `ui.ticker.start` | `right`: enters at the right edge; `left`: starts visible at the left |
| `ui.popup.autohide` | hide the popup after this long, e.g. `"20s"` (`"0s"` = never) |
| `ui.status.enabled` | small top-right pill with "thinking…" and notices |

Unknown keys are rejected, so typos get caught.

## AI providers

All providers use the OpenAI chat API. API keys come from environment variables and
never go in the config file.

| `provider` | Address | Key variable | Cost |
|---|---|---|---|
| `ollama` | `http://localhost:11434/v1` | — | **free**, local, private |
| `gemini` | Google AI Studio | `GEMINI_API_KEY` | **free tier** (daily limits) |
| `openrouter` | openrouter.ai | `OPENROUTER_API_KEY` | `openrouter/free` is **free** (rate-limited); paid models too |
| `routerai` | routerai.ru | `ROUTERAI_API_KEY` | paid |
| `requesty` | router.requesty.ai | `REQUESTY_API_KEY` | paid |
| `grok` | api.x.ai | `XAI_API_KEY` | paid (prepaid credits) |
| `openai_compat` | your `base_url` | your `api_key_env` (optional) | LM Studio, llama.cpp, vLLM, OpenAI… |

**Image mode needs a vision model**, e.g. `qwen2.5vl:7b` or `llama3.2-vision` in Ollama,
`gemini-3.8-flash`, or `deepseek/deepseek-v4.1-flash`. Text-only models such as `llama3.1`
work in OCR mode.

### Getting free keys

- **Ollama:** install from ollama.com, then run `ollama pull qwen2.5vl:7b`. No key.
- **Gemini:** aistudio.google.com → **Get API key**. No card needed. On the free tier,
  Google may use your inputs to improve its models.
- **OpenRouter:** openrouter.ai → **Keys** → **Create key**. Use the model `openrouter/free`,
  which picks a free model that can read images.

```bash
export GEMINI_API_KEY=...        # add to ~/.zshrc or ~/.bashrc to keep it
```

A provider whose key isn't set is skipped when you switch to it.

## Troubleshooting

| Problem | Fix |
|---|---|
| `failed to register … Accessibility` | macOS: allow peekai (or your terminal) in Privacy & Security → Accessibility, then restart it |
| `cannot capture display` | macOS: allow Screen Recording, then restart |
| `model "…" can't read images` | use a vision model, or switch to OCR mode (Ctrl+Shift+M) |
| `401 … (check the key in X_API_KEY)` | the key in that variable is wrong. OpenRouter says "Missing Authentication header" even for wrong keys |
| `context deadline exceeded` | the model is slow (local models while loading, "thinking" models): raise `ai.timeout` |
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
