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
