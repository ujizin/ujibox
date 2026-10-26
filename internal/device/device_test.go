package device

import (
	"strings"
	"testing"
)

var devs = []Device{
	{Platform: "android", ID: "emulator-5554", Name: "Pixel_9_Pro_Fold", AVD: "Pixel_9_Pro_Fold", State: StateReady},
	{Platform: "android", Name: "medium_phone", AVD: "medium_phone", State: StateOff},
	{Platform: "ios", ID: "AAA", Name: "iPhone 17", OS: "26.5", State: StateOff},
	{Platform: "ios", ID: "BBB", Name: "iPhone 17", OS: "27.0", State: StateOff},
	{Platform: "ios", ID: "CCC", Name: "iPhone 17 Pro", OS: "26.5", State: StateReady},
}

func TestMatch(t *testing.T) {
	aliases := map[string]string{"fold": "Pixel_9_Pro_Fold"}
	cases := map[string]string{
		"fold":           "Pixel_9_Pro_Fold",
		"emulator-5554":  "Pixel_9_Pro_Fold",
		"medium":         "medium_phone",
		"iphone 17 pro":  "iPhone 17 Pro",
		"iPhone 17@27.0": "iPhone 17",
		"BBB":            "iPhone 17",
	}
	for in, want := range cases {
		d, err := match(in, devs, aliases)
		if err != nil {
			t.Errorf("match(%q): %v", in, err)
			continue
		}
		if d.Name != want {
			t.Errorf("match(%q) = %s, want %s", in, d.Name, want)
		}
	}
	// Two shut-down "iPhone 17" simulators: ask, do not guess.
	if _, err := match("iPhone 17", devs, aliases); err == nil || !strings.Contains(err.Error(), "matches 2 devices") {
		t.Errorf("ambiguous name: err = %v", err)
	}
}

func TestParseSelectors(t *testing.T) {
	s := ParseSelectors([]string{"ios", "fold"}, true, false)
	if strings.Join(s.Platforms, ",") != "android,ios" || strings.Join(s.Names, ",") != "fold" {
		t.Fatalf("%+v", s)
	}
}

func TestIOSRuntimeVersion(t *testing.T) {
	if v, ok := iosRuntimeVersion("com.apple.CoreSimulator.SimRuntime.iOS-26-5"); !ok || v != "26.5" {
		t.Fatalf("got %q %v", v, ok)
	}
	if _, ok := iosRuntimeVersion("com.apple.CoreSimulator.SimRuntime.watchOS-12-0"); ok {
		t.Fatal("watchOS is not an iOS runtime")
	}
}

func TestHostTargets(t *testing.T) {
	for _, p := range []string{"desktop", "web"} {
		if got := DefaultTarget(p); got != p {
			t.Errorf("DefaultTarget(%s) = %s", p, got)
		}
		if got := TargetPlatform(p); got != p {
			t.Errorf("TargetPlatform(%s) = %s", p, got)
		}
	}
	if TargetPlatform(TargetDevice) != "ios" || TargetPlatform(TargetEmulator) != "android" {
		t.Error("device targets changed platform")
	}
}

func TestSelectionSplit(t *testing.T) {
	sel := ParseSelectors([]string{"desktop", "android", "fold", "web"}, false, false)
	dev, host := sel.Split()
	if strings.Join(dev.Platforms, ",") != "android" || strings.Join(dev.Names, ",") != "fold" || strings.Join(host, ",") != "desktop,web" {
		t.Errorf("Split = %+v %v", dev, host)
	}
	all := ParseSelectors([]string{"all"}, false, false)
	if strings.Join(all.Platforms, ",") != "android,ios" {
		t.Errorf("all = %v; it must mean the device platforms", all.Platforms)
	}
}

func TestResolveRejectsHost(t *testing.T) {
	m := &Manager{}
	_, err := m.Resolve(t.Context(), ParseSelectors([]string{"web"}, false, false), AllPlatforms)
	if err == nil || !strings.Contains(err.Error(), "not a device platform") {
		t.Errorf("Resolve(web) = %v", err)
	}
}

func TestPickWindow(t *testing.T) {
	w := func(n, layer int, on bool, wd, h float64) window {
		x := window{Number: n, Layer: layer, OnScreen: on, Alpha: 1}
		x.Bounds.Width, x.Bounds.Height = wd, h
		return x
	}
	got, err := pickWindow([]window{
		w(1, 0, false, 1000, 1000), // off screen
		w(2, 25, true, 1000, 1000), // a menu, not a normal window
		w(3, 0, true, 1728, 33),    // a thin bar
		w(4, 0, true, 800, 600),
		w(5, 0, true, 300, 200),
	})
	if err != nil || got.Number != 4 {
		t.Errorf("pickWindow = %d, %v", got.Number, err)
	}
	if _, err := pickWindow([]window{w(1, 0, false, 800, 600)}); err == nil {
		t.Error("no window on screen must fail")
	}
}
