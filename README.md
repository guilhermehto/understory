# pomodoro

A terminal pomodoro timer with Taskwarrior tracking and an audio-reactive spectrum visualizer. Big ASCII clock, classic 25/5/15 cycle, long break every 4 focus sessions. Stats and task links persist to `~/.pomodoro.json`.

## Build and run

```
go build -o pomodoro . && ./pomodoro
```

Or `go run .`. No cgo, no setup for the timer itself.

## Keys

- `space` start / pause, and start the next phase once one completes
- `s` skip phase
- `r` reset phase
- `t` open the task view
- `v` toggle the visualizer
- `q` quit

In the task view: `↑`/`↓` move, `enter` set current task, `d` mark done, `a` add, `e` edit, `t`/`esc` back.

## Flags

- `-work N` / `-short N` / `-long N` durations in minutes (default 25/5/15). Handy for testing, e.g. `-work 1`.

## Tasks

Needs the `task` CLI (`brew install task`). The task view lists your pending tasks by urgency. Pick one as the current task, mark tasks done, add new ones, edit description/project/tags.

Each completed focus session credits one pomodoro to the current task plus anything you marked done that session, stored locally in `~/.pomodoro.json`. Nothing is written back to Taskwarrior except the done/add/edit actions you take yourself.

## Visualizer (macOS 14.4+)

Press `v`. The app taps the system audio output directly through a Core Audio process tap — no loopback driver, no ffmpeg, nothing to install or reboot. Volume keys keep working and it hears exactly what you hear.

First press compiles a small Swift helper into `~/Library/Caches/pomodoro/` (needs the Xcode command line tools: `xcode-select --install`) and triggers the System Audio Recording permission prompt — allow it. Manage the permission later under System Settings → Privacy & Security → Screen & System Audio Recording.
