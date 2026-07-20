package main

import (
	"encoding/binary"
	"math"
	"math/cmplx"
	"strings"
	"testing"
)

// FFT of a pure sine at bin k must peak at bin k.
func TestFFTPeak(t *testing.T) {
	const n, k = 64, 5
	x := make([]complex128, n)
	for i := 0; i < n; i++ {
		x[i] = complex(math.Sin(2*math.Pi*float64(k)*float64(i)/float64(n)), 0)
	}
	fft(x)
	peak, best := 0, 0.0
	for i := 1; i < n/2; i++ {
		if mag := cmplx.Abs(x[i]); mag > best {
			best, peak = mag, i
		}
	}
	if peak != k {
		t.Fatalf("peak bin = %d, want %d", peak, k)
	}
}

func TestHSVToHex(t *testing.T) {
	cases := map[float64]string{0: "#ff0000", 120: "#00ff00", 240: "#0000ff"}
	for h, want := range cases {
		if got := hsvToHex(h, 1, 1); got != want {
			t.Errorf("hsvToHex(%g) = %s, want %s", h, got, want)
		}
	}
}

func TestRenderSpectrumShape(t *testing.T) {
	levels := []float64{1, 0} // col0 full height, col1 empty
	out := stripANSItest(renderSpectrum(levels, 2, 4))
	rows := strings.Split(out, "\n")
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	if r := []rune(rows[0]); r[0] != '█' { // top of a full column
		t.Errorf("top of full column = %q, want █", string(r[0]))
	}
	for _, row := range rows { // empty column stays blank everywhere
		if r := []rune(row); r[1] != ' ' {
			t.Errorf("empty column not blank: %q", string(r[1]))
		}
	}
}

// TestPipelineTone drives the full PCM-parse → FFT → band pipeline with a
// synthetic 1kHz f32le tone and asserts the loudest band is where 1kHz lands.
func TestPipelineTone(t *testing.T) {
	const rate, ns = 44100, 44100
	data := make([]byte, ns*4)
	for i := range ns {
		s := float32(math.Sin(2 * math.Pi * 1000 * float64(i) / rate))
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(s))
	}

	samples := make([]float64, ns)
	for i := range ns {
		samples[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:])))
	}

	an := newAnalyzer(rate)
	acc := make([]float64, numBands)
	for off := 0; off+fftSize <= ns; off += fftSize {
		for i, v := range an.bands(samples[off : off+fftSize]) {
			acc[i] += v
		}
	}
	peak, best := 0, 0.0
	for i, v := range acc {
		if v > best {
			best, peak = v, i
		}
	}
	// 1kHz over a 40Hz..16kHz log scale of 64 bands lands at band ~34.
	if peak < 30 || peak > 39 {
		t.Fatalf("1kHz peak band = %d, want ~34", peak)
	}
}

func stripANSItest(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}
