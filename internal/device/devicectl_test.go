package device

import (
	"encoding/json"
	"testing"
)

// The fields that `devicectl list devices --json-output` writes, with made-up
// values: Xcode 27 uses "properties", older versions use the *Properties keys.
const devicectlFixture = `{"result": {"devices": [
  {"capabilities": [{"featureIdentifier": "com.apple.coredevice.feature.connectdevice"}],
   "properties": {"connection": {"pairingState": "paired", "transportType": "wired"},
     "hardware": {"udid": "00000000-0000000000000001", "platform": "iOS", "reality": "physical", "marketingName": "iPhone 15"},
     "software": {"osVersionNumber": {"components": [27, 0, 0], "stringValue": "27.0"}},
     "state": {"name": "Test Phone", "developerModeStatus": {"enabled": {"mode": 1}}}}},
  {"capabilities": [],
   "properties": {"connection": {"pairingState": "paired"},
     "hardware": {"udid": "00000000-0000000000000002", "platform": "iOS", "reality": "physical"},
     "state": {"name": "Away Phone", "developerModeStatus": {"enabled": {"mode": 1}}}}},
  {"capabilities": [{"featureIdentifier": "x"}],
   "properties": {"connection": {"pairingState": "unpaired"},
     "hardware": {"udid": "00000000-0000000000000003", "platform": "iOS", "reality": "physical"},
     "state": {"name": "New Phone"}}},
  {"capabilities": [{"featureIdentifier": "x"}],
   "hardwareProperties": {"udid": "00000000-0000000000000004", "platform": "iOS", "reality": "physical", "marketingName": "iPad"},
   "deviceProperties": {"name": "Old Xcode iPad", "osVersionNumber": "18.2", "developerModeStatus": "disabled"},
   "connectionProperties": {"pairingState": "paired"}},
  {"capabilities": [{"featureIdentifier": "x"}],
   "properties": {"connection": {"pairingState": "paired"},
     "hardware": {"udid": "SIM-UDID", "platform": "iOS", "reality": "simulated"},
     "state": {"name": "iPhone 17"}}},
  {"capabilities": [{"featureIdentifier": "x"}],
   "properties": {"connection": {"pairingState": "paired"},
     "hardware": {"udid": "MAC-UDID", "platform": "macOS", "reality": "physical"},
     "state": {"name": "A Mac"}}}
]}}`

func TestDevicectlDevices(t *testing.T) {
	var v devicectlList
	if err := json.Unmarshal([]byte(devicectlFixture), &v); err != nil {
		t.Fatal(err)
	}
	got := map[string]Device{}
	for _, d := range v.devices() {
		got[d.Name] = d
	}
	if len(got) != 4 {
		t.Fatalf("want the 4 physical iOS devices, got %+v", got)
	}
	want := map[string]string{
		"Test Phone":     StateReady,
		"Away Phone":     StateOffline,
		"New Phone":      StateUnauthorized,
		"Old Xcode iPad": StateNoDevMode,
	}
	for name, state := range want {
		if got[name].State != state {
			t.Errorf("%s: state = %q, want %q", name, got[name].State, state)
		}
	}
	p := got["Test Phone"]
	if p.Kind != KindPhysical || p.ID != "00000000-0000000000000001" || p.OS != "27.0" || p.Model != "iPhone 15" {
		t.Errorf("Test Phone = %+v", p)
	}
	if p.Target() != TargetDevice {
		t.Errorf("target = %q", p.Target())
	}
	if !p.Default() {
		t.Error("a ready phone on a cable must be in the default set")
	}
	wifi := p
	wifi.Wireless = true
	if wifi.Default() {
		t.Error("a phone on the network must be named")
	}
	if got["Old Xcode iPad"].OS != "18.2" {
		t.Errorf("old format: %+v", got["Old Xcode iPad"])
	}
}

func TestDevicectlReason(t *testing.T) {
	out := "02:00:00  Acquired tunnel connection to device.\nERROR: The device is locked. (com.apple.dt.CoreDeviceError error 1.)\n"
	if got := devicectlReason(out, nil); got != "The device is locked. (com.apple.dt.CoreDeviceError error 1.)" {
		t.Errorf("reason = %q", got)
	}
}
