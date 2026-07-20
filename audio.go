package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/cmplx"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	fftSize  = 1024 // ~46 frames/sec at 48kHz
	numBands = 64
)

//go:embed tap.swift
var tapSource []byte

//go:embed tap-info.plist
var tapPlist []byte

// audioFrame carries one analysed spectrum frame, or a terminal error.
type audioFrame struct {
	levels []float64
	err    error
}

// tapHelper returns the compiled system-audio capture helper, building it with
// swiftc on first use or when the embedded source changed. The binary path is
// stable so the System Audio Recording grant keyed to it survives reruns.
func tapHelper() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cacheRoot, "understory")
	bin := filepath.Join(dir, "understory")
	src := filepath.Join(dir, "tap.swift")
	plist := filepath.Join(dir, "tap-info.plist")

	if old, err := os.ReadFile(src); err == nil && bytes.Equal(old, tapSource) {
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(src, tapSource, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(plist, tapPlist, 0o644); err != nil {
		return "", err
	}
	out, err := exec.Command("swiftc", "-O", src, "-o", bin,
		"-Xlinker", "-sectcreate", "-Xlinker", "__TEXT",
		"-Xlinker", "__info_plist", "-Xlinker", plist).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return "", errors.New("compiling audio helper: " + msg)
		}
		return "", fmt.Errorf("compiling audio helper (needs Xcode command line tools): %w", err)
	}
	return bin, nil
}

// captureAudio streams the system-audio mix from the tap helper, analyses each
// block into log-spaced spectrum bands, and sends frames on out until ctx is
// cancelled or the helper dies. Non-blocking sends drop frames if the UI is
// busy — a visualizer wants the latest frame, not a backlog.
func captureAudio(ctx context.Context, out chan<- audioFrame) {
	defer close(out)

	bin, err := tapHelper()
	if err != nil {
		out <- audioFrame{err: err}
		return
	}
	cmd := exec.CommandContext(ctx, bin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		out <- audioFrame{err: err}
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		out <- audioFrame{err: err}
		return
	}
	die := func(readErr error) {
		werr := cmd.Wait()
		if ctx.Err() != nil {
			return // user toggled off — not an error
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && werr != nil {
			msg = werr.Error()
		}
		if msg == "" {
			msg = readErr.Error()
		}
		out <- audioFrame{err: errors.New(msg)}
	}

	// 4-byte header: the tap's native sample rate (whatever the output device
	// runs at), then endless mono f32le PCM.
	var hdr [4]byte
	if _, err := io.ReadFull(stdout, hdr[:]); err != nil {
		die(err)
		return
	}
	an := newAnalyzer(float64(binary.LittleEndian.Uint32(hdr[:])))
	raw := make([]byte, fftSize*4)
	samples := make([]float64, fftSize)

	for {
		if _, err := io.ReadFull(stdout, raw); err != nil {
			die(err)
			return
		}
		for i := range fftSize {
			samples[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])))
		}
		select {
		case out <- audioFrame{levels: an.bands(samples)}:
		default: // UI not ready; drop this frame
		}
	}
}

// analyzer holds the Hann window and an adaptive gain so bars auto-scale to the
// current volume without a hand-tuned dB floor.
type analyzer struct {
	window     []float64
	sampleRate float64
	runningMax float64
}

func newAnalyzer(rate float64) *analyzer {
	w := make([]float64, fftSize)
	for i := range w {
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize-1))
	}
	return &analyzer{window: w, sampleRate: rate, runningMax: 1e-6}
}

func (a *analyzer) bands(samples []float64) []float64 {
	buf := make([]complex128, fftSize)
	for i := range fftSize {
		buf[i] = complex(samples[i]*a.window[i], 0)
	}
	fft(buf)

	half := fftSize / 2
	binHz := a.sampleRate / float64(fftSize)
	const fmin, fmax = 40.0, 16000.0

	bands := make([]float64, numBands)
	frameMax := 1e-9
	for b := range numBands {
		f0 := fmin * math.Pow(fmax/fmin, float64(b)/numBands)
		f1 := fmin * math.Pow(fmax/fmin, float64(b+1)/numBands)
		lo, hi := int(f0/binHz), int(f1/binHz)
		if lo < 1 {
			lo = 1
		}
		if hi <= lo {
			hi = lo + 1
		}
		if hi > half {
			hi = half
		}
		sum := 0.0
		for i := lo; i < hi; i++ {
			sum += cmplx.Abs(buf[i])
		}
		v := sum / float64(hi-lo)
		bands[b] = v
		if v > frameMax {
			frameMax = v
		}
	}

	// adaptive gain: jump up instantly, decay slowly toward quiet passages.
	if frameMax > a.runningMax {
		a.runningMax = frameMax
	} else {
		a.runningMax = a.runningMax*0.995 + frameMax*0.005
	}
	if a.runningMax < 1e-6 {
		a.runningMax = 1e-6
	}
	for b := range bands {
		l := bands[b] / a.runningMax
		if l > 1 {
			l = 1
		}
		bands[b] = math.Sqrt(l) // perceptual curve
	}
	return bands
}

// fft is an in-place iterative radix-2 Cooley-Tukey transform; len(x) must be a
// power of two.
func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wl := cmplx.Rect(1, ang)
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for j := range length / 2 {
				u := x[i+j]
				v := x[i+j+length/2] * w
				x[i+j] = u + v
				x[i+j+length/2] = u - v
				w *= wl
			}
		}
	}
}
