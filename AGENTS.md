# AGENTS.md

understory: macOS Pomodoro TUI (Go, bubbletea/lipgloss). Taskwarrior integration, /etc/hosts site blocking, Core Audio spectrum visualizer.

## Build / test / run

```
go build -o understory . && ./understory   # or: go run .
go test ./...
```

No cgo. Single package at repo root. `-work 1` makes short sessions for manual testing.

## File map

- `main.go` — model, Update/View, rendering (bar, big digits, spectrum), persistence (`~/.understory.json`)
- `task.go` — Taskwarrior CLI wrapper (`task` binary, JSON export)
- `block.go` — hosts-file blocking via root-owned helper + sudoers rule
- `audio.go` + `tap.swift` — Core Audio process tap; Swift helper compiled at runtime into `~/Library/Caches/understory/`
- `heatmap.go`, `quotes.go` — weekly heatmap, idle quotes
- `demo.tape` — vhs recording script; full regen recipe in its header comments

## Conventions

- `ponytail:` comments mark deliberate simplifications with their ceiling and upgrade path. Keep them; add one when you cut a corner.
- State lives in `~/.understory.json` (`persisted` struct in main.go). Old stats-only format is migrated in `decodeState` — don't break that.
- Never run understory itself with sudo. Blocking goes through `/usr/local/libexec/understory-block`.

## Traps (learned the hard way)

- **NEVER override `$HOME` to sandbox state.** The audio tap helper lives at `~/Library/Caches/understory/understory` and the macOS System Audio Recording (TCC) grant is keyed to that binary path. New path = rebuild = permission prompt = flat bars. Instead: `mv ~/.understory.json` aside, seed a demo one, restore after.
- **Taskwarrior sandbox:** the app inherits env, so `TASKRC=/tmp/demo.taskrc TASKDATA=/tmp/demo-taskdata` isolates the task view from real tasks. Seed an empty blocklist too, or a demo focus session edits the real `/etc/hosts`.
- **Layout:** timer view has 23 fixed rows; spectrum gets `clamp(m.height-23, 0, 8)` rows (commit 0478fcf / PR #9). Full 8-row spectrum + title needs ≥31 terminal rows.

## Recording demo.gif with vhs

Known-good dims: 1280x720 (exact 16:9), FontSize 15, Padding 16 → ~32 rows, everything fits.

### Env leakage kills color

The agent harness shell leaks `CI=1` and `NO_COLOR=1` into vhs's ttyd session. termenv treats unrecognized CI as a dumb terminal → Ascii profile → lipgloss strips ALL color. "Missing glyphs" are a symptom, not a font issue: the bar's `░` empty cells are styled near-background dark (#2a2b36); unstyled they render bright checkerboard.

Fix (both belts):
1. In demo.tape's Hide block, before launching: `Type "unset CI NO_COLOR; export COLORTERM=truecolor ..."`
2. Launch clean: `env -u CI -u NO_COLOR TERM=xterm-256color vhs demo.tape`

### Diagnosing monochrome recordings (reusable)

1. Probe tape: raw `printf '\e[31m...\e[38;2;R;G;Bm...'` + `env | grep -i color`. Raw ANSI renders colored → vhs/ttyd/font are fine; suspect app-side profile detection.
2. Pixel-sample extracted frames (`ffmpeg -ss N -i demo.gif -vframes 1` + avg a 16x16 crop). R==G==B exactly → color truly stripped, not a pale palette.
3. Definitive: run the binary under a python pty (must set TIOCSWINSZ or bubbletea renders nothing) and count `\x1b[38;2;` occurrences in captured bytes. CI set → 0; CI unset → thousands.

### Visualizer during recording

Needs real system audio, audible on the machine: play a generated tone (ffmpeg lavfi: 3 sines with different tremolo rates + pink noise, ~25s, `afplay` in the tape's Hide block). Exact command in demo.tape's header.

### Verify after any re-record

Extract frames at ~7s (clock+viz), ~10.5s (tasks), ~15s (heatmap); check: blue clock, cyan→pink spectrum gradient, title "U N D E R S T O R Y" + quote visible (not clipped), footer ends with "q quit", `ffprobe` reports 1280x720.
