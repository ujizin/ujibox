package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"

	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/store"
)

func TestViewLists(t *testing.T) {
	m := &model{
		deps:     Deps{Title: "Sample", Group: "main"},
		selected: map[string]bool{},
		width:    120, height: 24,
		devices: []device.Device{
			{Platform: "android", ID: "emulator-5554", Name: "Pixel_9", State: device.StateReady, OS: "16"},
			{Platform: "ios", ID: "AAA", Name: "iPhone 17", State: device.StateOff, OS: "26.5"},
		},
		builds: []*store.Manifest{
			{ID: "01AAAAAAAAAAAAAAAAAAAAAAAA", App: store.AppRef{ID: "s", Name: "Sample"}, Platform: "android",
				Git: store.Git{Branch: "main", SHA: "abcdef0123"}, Version: store.Version{Name: "1.2.3"}, CreatedAt: time.Now()},
		},
	}
	m.selected[key(m.devices[0])] = true
	v := m.View()
	for _, want := range []string{"Pixel_9", "iPhone 17", "1.2.3", "← HEAD", "1 selected"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	if got := m.targets(); len(got) != 1 || got[0] != "emulator-5554" {
		t.Errorf("targets = %v", got)
	}
	t.Log("\n" + v)
}

func TestChooserKeys(t *testing.T) {
	m := chooser{title: "Where?", options: []Option{{Label: "a"}, {Label: "b"}}}
	press := func(m chooser, k tea.KeyMsg) chooser {
		out, _ := m.Update(k)
		return out.(chooser)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cur != 1 {
		t.Fatalf("down: cur = %d", m.cur)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cur != 0 {
		t.Fatalf("down wraps: cur = %d", m.cur)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cur != 1 {
		t.Fatalf("up wraps: cur = %d", m.cur)
	}
	if done := press(m, tea.KeyMsg{Type: tea.KeyEnter}); !done.done || done.cur != 1 {
		t.Errorf("enter: %+v", done)
	}
	if c := press(m, tea.KeyMsg{Type: tea.KeyEsc}); !c.canceled {
		t.Error("esc must cancel")
	}
	if n := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}); !n.done || n.cur != 0 {
		t.Errorf("1: %+v", n)
	}
}
