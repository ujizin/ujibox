package tui

import (
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
