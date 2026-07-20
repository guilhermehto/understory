package main

import (
	"testing"
	"time"
)

func TestCadence(t *testing.T) {
	// Simulate: 4 focus sessions → long break every 4th, short otherwise.
	m := model{ph: focus}
	want := []phase{shortBreak, shortBreak, shortBreak, longBreak}
	for i, w := range want {
		m.ph = focus
		m.cycle++ // a focus just completed
		if got := m.next(); got != w {
			t.Fatalf("focus #%d: next=%v want %v", m.cycle, got, w)
		}
		_ = i
		m.ph = w // pretend we took the break; break always → focus
		if got := m.next(); got != focus {
			t.Fatalf("after %v: next=%v want focus", w, got)
		}
	}
}

func TestFmtDuration(t *testing.T) {
	cases := map[time.Duration]string{
		25 * time.Minute:                   "25:00",
		90 * time.Second:                   "01:30",
		0:                                  "00:00",
		-5 * time.Second:                   "00:00",
		time.Minute + 59500*time.Millisecond: "02:00", // rounds to nearest second
	}
	for d, want := range cases {
		if got := fmtDuration(d); got != want {
			t.Errorf("fmtDuration(%v)=%q want %q", d, got, want)
		}
	}
}

func TestHexLerp(t *testing.T) {
	if got := hexLerp("#000000", "#ffffff", 0); got != "#000000" {
		t.Errorf("t=0 got %s", got)
	}
	if got := hexLerp("#000000", "#ffffff", 1); got != "#ffffff" {
		t.Errorf("t=1 got %s", got)
	}
	if got := hexLerp("#000000", "#ffffff", 0.5); got != "#808080" {
		t.Errorf("t=0.5 got %s", got)
	}
}

func TestCreditTasksCurrentPlusDoneBlock(t *testing.T) {
	m := model{
		curUUID:   "a",
		curDesc:   "A",
		doneBlock: map[string]string{"b": "B", "a": "A"}, // "a" also current
		links:     map[string]taskLink{},
	}
	m.creditTasks()
	if got := m.links["a"].Pomodoros; got != 1 { // deduped, not double-counted
		t.Errorf("a pomodoros=%d want 1", got)
	}
	if got := m.links["b"].Pomodoros; got != 1 {
		t.Errorf("b pomodoros=%d want 1", got)
	}
	if len(m.doneBlock) != 0 {
		t.Errorf("doneBlock not cleared: %v", m.doneBlock)
	}
	m.creditTasks() // second focus, current still "a"
	if got := m.links["a"].Pomodoros; got != 2 {
		t.Errorf("a pomodoros after 2nd=%d want 2", got)
	}
	if got := m.links["b"].Pomodoros; got != 1 {
		t.Errorf("b pomodoros after 2nd=%d want 1 (b not current)", got)
	}
}

func TestDecodeStateMigratesOldFormat(t *testing.T) {
	old := []byte(`{"total":10,"today":3,"date":"` + today() + `"}`)
	s, links, sess := decodeState(old)
	if s.Total != 10 || s.Today != 3 {
		t.Errorf("old-format stats lost: %+v", s)
	}
	if links == nil || len(links) != 0 {
		t.Errorf("links should be empty non-nil, got %v", links)
	}
	if sess != nil {
		t.Errorf("old format should carry no session, got %+v", sess)
	}

	newFmt := []byte(`{"stats":{"total":5,"today":2,"date":"` + today() + `"},"links":{"u1":{"description":"x","pomodoros":4,"last_worked":"` + today() + `"}},"session":{"phase":1,"remaining_sec":90,"cycle":2}}`)
	s2, links2, sess2 := decodeState(newFmt)
	if s2.Total != 5 || s2.Today != 2 {
		t.Errorf("new-format stats wrong: %+v", s2)
	}
	if links2["u1"].Pomodoros != 4 {
		t.Errorf("new-format links wrong: %+v", links2)
	}
	if sess2 == nil || sess2.Phase != shortBreak || sess2.RemainingSec != 90 || sess2.Cycle != 2 {
		t.Errorf("session wrong: %+v", sess2)
	}
}

// Quit acts as pause: a saved session resumes paused with its remaining time,
// clamped to the (possibly shrunk) phase duration.
func TestRestoreSession(t *testing.T) {
	base := model{durations: [3]time.Duration{25 * time.Minute, 5 * time.Minute, 15 * time.Minute}, ph: focus, status: idle}
	base.remaining = base.dur(focus)

	m := base
	m.restore(&session{Phase: shortBreak, RemainingSec: 90, Cycle: 2})
	if m.ph != shortBreak || m.status != paused || m.remaining != 90*time.Second || m.cycle != 2 {
		t.Errorf("restore = phase %v status %v remaining %v cycle %d", m.ph, m.status, m.remaining, m.cycle)
	}

	m = base
	m.restore(&session{Phase: focus, RemainingSec: 3600, Cycle: 0}) // saved before -work shrank
	if m.remaining != 25*time.Minute {
		t.Errorf("remaining not clamped: %v", m.remaining)
	}

	m = base
	m.restore(nil)
	m.restore(&session{Phase: 9, RemainingSec: 60})
	m.restore(&session{Phase: focus, RemainingSec: 0})
	if m.ph != focus || m.status != idle || m.remaining != 25*time.Minute {
		t.Errorf("invalid sessions mutated model: %+v", m)
	}
}

func TestDateRollResetsTodayKeepsTotal(t *testing.T) {
	m := model{stats: stats{Total: 10, Today: 3, Date: "2000-01-01"}}
	m.rollDate()
	if m.stats.Today != 0 {
		t.Errorf("today not reset: %d", m.stats.Today)
	}
	if m.stats.Total != 10 {
		t.Errorf("total changed: %d", m.stats.Total)
	}
	if m.stats.Date != today() {
		t.Errorf("date not rolled: %s", m.stats.Date)
	}
}
