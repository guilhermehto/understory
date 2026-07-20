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
- `-audio <device>` visualizer input by index or name substring. Default order: BlackHole, then mic, then first input.

## Tasks

Needs the `task` CLI (`brew install task`). The task view lists your pending tasks by urgency. Pick one as the current task, mark tasks done, add new ones, edit description/project/tags.

Each completed focus session credits one pomodoro to the current task plus anything you marked done that session, stored locally in `~/.pomodoro.json`. Nothing is written back to Taskwarrior except the done/add/edit actions you take yourself.

## Visualizer (macOS)

The visualizer reads audio through `ffmpeg` (`brew install ffmpeg`). To react to the music you're playing instead of your mic, you need a loopback device. macOS has no built-in one, so use BlackHole.

1. Install and reboot. The driver only loads after a restart.
   ```
   brew install --cask blackhole-2ch
   ```
2. Open Audio MIDI Setup (Applications > Utilities), click `+` > Create Multi-Output Device, and tick both your normal output (speakers/headphones) and BlackHole 2ch. This copies audio to BlackHole while you still hear it.
3. Set that Multi-Output Device as your system output in System Settings > Sound, or the menu-bar volume control.
4. Run the app and press `v`. macOS treats the loopback as a microphone, so allow the Microphone prompt on first use. It reads only the system audio routed through BlackHole, not your actual mic.

The app auto-selects BlackHole when it's present, otherwise it falls back to the mic.

A per-app output menu (Zoom, Spotify) won't list the Multi-Output Device. macOS hides those from app pickers. Leave apps on "same as system" and select the device system-wide.

Verify BlackHole is detected:

```
ffmpeg -f avfoundation -list_devices true -i "" 2>&1 | grep -A20 "audio devices"
```
