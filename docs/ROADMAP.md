# peekai roadmap

Hotkey → screenshot → (OCR) → AI → on-top popup / ticker.

Each feature is **one branch → one PR** with small [conventional commits](https://www.conventionalcommits.org/).
Branch name = `<type>/<feature>`. Every PR must pass `lint` (golangci-lint on Linux/macOS/Windows + commitlint).

Allowed commit scopes: `config capture ocr ai pipeline hotkey app ui popup ticker tray ci deps docs release`.

## Architecture (target)

```
cmd/peekai/main.go          wiring only
internal/config             config.json load / defaults / validate / reload
internal/capture            Capturer interface + screenshot impl
internal/ocr                Engine interface + tesseract_cli, llm
internal/ai                 Provider interface + openai_compat, anthropic
internal/pipeline           Capture → [OCR] → AI, context-aware
internal/hotkey             Registrar interface + golang.design/x/hotkey impl
internal/app                Controller: action → behaviour, cancel, state
internal/ui                 Presenter interface + gogui impl (popup, ticker, tray)
```

Rule: packages depend on interfaces; only `cmd/` and `internal/app` touch concrete types.
Swapping a backend = add one file + one registry line + change `config.json`.

---

## 0. Bootstrap — `ci/lint-workflow`
- [x] `chore: init go module`
- [x] `ci: add golangci-lint and commitlint workflow`
- [x] `docs: add roadmap`

## 1. Config — `feat/config`
- `feat(config): add config schema with defaults`
- `feat(config): load config.json and write default on first run`
- `feat(config): validate mode, output, provider and hotkeys`
- `test(config): cover defaults, overrides and invalid input`

Done when: `peekai` creates `config.json` next to its executable (current dir under `go run`) and errors clearly on bad values.

## 2. Screen capture — `feat/capture`
- `feat(capture): add Capturer interface`
- `feat(capture): implement screenshot capture of active/all/indexed display`
- `feat(capture): downscale image to max_image_px`
- `feat(app): add --once flag that saves capture.png`

Done when: `peekai --once` writes a PNG on all 3 OSes (macOS: Screen Recording permission documented).

## 3. AI providers — `feat/ai`
- `feat(ai): add Provider interface and Request type`
- `feat(ai): add openai-compatible chat completions client`
- `feat(ai): add ollama, routerai and grok presets`
- `feat(config): default to ollama provider preset`
- `test(ai): cover requests, auth, errors and presets with httptest`

Later: more presets (OpenAI, OpenRouter, Groq, Gemini) are one line each; Anthropic needs its own client.

## 4. Pipeline — `feat/pipeline`
- `feat(pipeline): run capture → ai in image mode`
- `feat(pipeline): render prompt as text/template with Text, Mode, Time`
- `feat(app): print AI result in --once mode`
- `test(pipeline): verify stage order and cancellation with fakes`

Done when: `peekai --once` prints an answer from a local Ollama.

## 5. OCR — `feat/ocr`
- `feat(ocr): add Engine interface`
- `feat(ocr): add tesseract cli engine`
- `feat(ocr): add llm engine (vision model extracts text)`
- `feat(pipeline): support ocr mode`
- `test(ocr): cover tesseract output parsing`

## 6. Hotkeys & controller — `feat/hotkeys`
- `feat(hotkey): parse shortcut strings like Ctrl+Shift+S`
- `feat(hotkey): register global hotkeys from config`
- `feat(ui): add Presenter interface and console presenter`
- `feat(app): add controller for all hotkey actions`
- `feat(app): run as hotkey listener when started without -once`
- `test: cover shortcut parsing and controller actions`

Done when: hotkeys run the pipeline and print results in the terminal (console presenter; real windows in 8–9).
Esc is not a global hotkey on purpose (it would steal Esc from every app); `hide_all` cancels a running request.

## 7–9. Overlay UI — `feat/ui`
Spike result: go-gui draws the windows (frameless, transparent, text, animation) but has no
always-on-top, positioning, click-through or native handle, and its `Show()` activates the app
(switches Spaces away from fullscreen apps). `internal/ui/ontop` fills the gap per OS, finding our
windows by title: macOS (Cocoa: status level, all Spaces incl. other apps' fullscreen, accessory app,
show without activating), Windows (Win32 topmost/tool/no-activate/click-through), X11 (EWMH above/sticky,
XFixes input shape; go-gui forced to X11).

- `feat(capture): capture the display under the mouse`
- `feat(ui): add on-top add-on for macOS, Windows and X11`
- `feat(ui): add go-gui overlay with popup, running line and status`
- `feat(app): show results in the overlay, add -console flag`

Done when: popup and running line appear only on their hotkeys, over the current app (also fullscreen)
on the display under the mouse, without stealing focus. Verified on macOS; Windows/X11 compile-checked only.

## 10. Tray & reload — `feat/tray`
- `feat(tray): tray icon with Reload config and Quit`
- `feat(config): reload config without restart`

## 11. Release — `ci/release`
- `docs: add README with setup, permissions and AI provider guide`
- `ci: add build matrix for macOS, Windows and Linux`
- `ci(release): publish binaries with goreleaser on tag`

## Later
- `feat(ocr): native Windows.Media.Ocr and Apple Vision engines`
- `feat(ai): fallback chain of providers`
- `feat(capture): region selection`
- `feat(hotkey): Wayland portal support`
