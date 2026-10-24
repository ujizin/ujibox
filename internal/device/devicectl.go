package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/proc"
)

// Physical iPhones and iPads go through `xcrun devicectl` (Xcode 15 or
// later). Screenshots and screen recording need Xcode 27 or later.

var errNoDevicectl = errors.New("devicectl not found: physical iOS devices need Xcode 15 or later")

// HasDevicectl reports whether the Xcode toolchain has devicectl.
func (t Tools) HasDevicectl() bool {
	return t.DeveloperDir != "" && fileExists(filepath.Join(t.DeveloperDir, "usr", "bin", "devicectl"))
}

func (m *Manager) devicectl(ctx context.Context, args ...string) (string, error) {
	if !m.Tools.HasDevicectl() {
		return "", errNoDevicectl
	}
	out, err := proc.Combined(ctx, m.Tools.XcodeEnv(), "xcrun", append([]string{"devicectl"}, args...)...)
	if err != nil {
		return out, fmt.Errorf("devicectl %s: %s", args[0]+" "+args[1], devicectlReason(out, err))
	}
	return out, nil
}

// devicectlJSON runs devicectl with --json-output and decodes the result.
func (m *Manager) devicectlJSON(ctx context.Context, v any, args ...string) error {
	f, err := os.CreateTemp("", "uji-devicectl-*.json")
	if err != nil {
		return err
	}
	f.Close()
	defer os.Remove(f.Name())
	if _, err := m.devicectl(ctx, append(args, "--quiet", "--json-output", f.Name())...); err != nil {
		return err
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// devicectlReason picks the line of devicectl output that explains a failure.
func devicectlReason(out string, err error) string {
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "ERROR:") {
			return strings.TrimSpace(strings.TrimPrefix(t, "ERROR:"))
		}
	}
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "Error:") || strings.HasPrefix(t, "error:") {
			return strings.TrimSpace(t[len("error:"):])
		}
	}
	if t := strings.TrimSpace(out); t != "" {
		return lastLine(t)
	}
	return err.Error()
}

// listIOSDevices returns the paired physical iPhones and iPads. Without
// devicectl it returns nothing: simulators still work.
func (m *Manager) listIOSDevices(ctx context.Context) ([]Device, error) {
	if !m.Tools.HasDevicectl() {
		return nil, nil
	}
	var v devicectlList
	if err := m.devicectlJSON(ctx, &v, "list", "devices"); err != nil {
		return nil, err
	}
	return v.devices(), nil
}

// devicectlList is the part of `devicectl list devices --json-output` that
// uji reads. Xcode 27 moved the fields into "properties"; older versions
// have hardwareProperties, deviceProperties and connectionProperties.
type devicectlList struct {
	Result struct {
		Devices []struct {
			Capabilities []struct {
				FeatureIdentifier string `json:"featureIdentifier"`
			} `json:"capabilities"`
			Properties struct {
				Connection struct {
					PairingState  string `json:"pairingState"`
					TransportType string `json:"transportType"`
				} `json:"connection"`
				Hardware struct {
					UDID          string `json:"udid"`
					Platform      string `json:"platform"`
					Reality       string `json:"reality"`
					MarketingName string `json:"marketingName"`
				} `json:"hardware"`
				Software struct {
					OSVersion json.RawMessage `json:"osVersionNumber"`
				} `json:"software"`
				State struct {
					Name          string          `json:"name"`
					DeveloperMode json.RawMessage `json:"developerModeStatus"`
				} `json:"state"`
			} `json:"properties"`
			HardwareProperties struct {
				UDID          string `json:"udid"`
				Platform      string `json:"platform"`
				Reality       string `json:"reality"`
				MarketingName string `json:"marketingName"`
			} `json:"hardwareProperties"`
			DeviceProperties struct {
				Name          string `json:"name"`
				OSVersion     string `json:"osVersionNumber"`
				DeveloperMode string `json:"developerModeStatus"`
			} `json:"deviceProperties"`
			ConnectionProperties struct {
				PairingState  string `json:"pairingState"`
				TransportType string `json:"transportType"`
			} `json:"connectionProperties"`
		} `json:"devices"`
	} `json:"result"`
}

func (v devicectlList) devices() []Device {
	var out []Device
	for _, d := range v.Result.Devices {
		p, hw, dp := d.Properties, d.HardwareProperties, d.DeviceProperties
		reality := first(p.Hardware.Reality, hw.Reality)
		platform := first(p.Hardware.Platform, hw.Platform)
		udid := first(p.Hardware.UDID, hw.UDID)
		// Simulators come from simctl; Macs, watches and TVs are not targets.
		if reality != "physical" || platform != "iOS" || udid == "" {
			continue
		}
		dev := Device{
			Platform: config.IOS,
			Kind:     KindPhysical,
			ID:       udid,
			Name:     first(p.State.Name, dp.Name),
			OS:       first(osVersion(p.Software.OSVersion), dp.OSVersion),
			Model:    first(p.Hardware.MarketingName, hw.MarketingName),
			State:    StateReady,
			// "wired" is a cable; "localNetwork" is a device on the same network.
			Wireless: first(p.Connection.TransportType, d.ConnectionProperties.TransportType) != "wired",
		}
		devMode := dp.DeveloperMode
		if len(p.State.DeveloperMode) > 0 {
			devMode = developerMode(p.State.DeveloperMode)
		}
		switch {
		case first(p.Connection.PairingState, d.ConnectionProperties.PairingState) != "paired":
			dev.State = StateUnauthorized
		case devMode == "disabled":
			dev.State = StateNoDevMode
		case len(d.Capabilities) == 0:
			// A paired device that is not reachable now lists no capabilities.
			dev.State = StateOffline
		}
		out = append(out, dev)
	}
	return out
}

// osVersion reads osVersionNumber: {"stringValue": "27.0.1", ...} in Xcode 27,
// or a plain string.
func osVersion(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var v struct {
		StringValue string `json:"stringValue"`
	}
	if json.Unmarshal(raw, &v) == nil {
		return v.StringValue
	}
	return ""
}

// developerMode reads developerModeStatus: {"enabled": {...}} in Xcode 27,
// or a plain string before.
func developerMode(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) == nil {
		for k := range m {
			return k
		}
	}
	return ""
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (m *Manager) installDevice(ctx context.Context, d Device, app string) error {
	_, err := m.devicectl(ctx, "device", "install", "app", "--device", d.ID, app)
	return err
}

func (m *Manager) launchDevice(ctx context.Context, d Device, bundleID string) error {
	_, err := m.devicectl(ctx, "device", "process", "launch", "--device", d.ID, "--terminate-existing", bundleID)
	return err
}

func (m *Manager) uninstallDevice(ctx context.Context, d Device, bundleID string) error {
	_, err := m.devicectl(ctx, "device", "uninstall", "app", "--device", d.ID, bundleID)
	return err
}

// openURLDevice launches the app with the URL. A physical device has no
// "open this URL" command, so uji needs the app: the one given, or Safari,
// which opens web links and asks before it opens another app's scheme.
func (m *Manager) openURLDevice(ctx context.Context, d Device, url, bundleID string) error {
	if bundleID == "" {
		bundleID = "com.apple.mobilesafari"
	}
	_, err := m.devicectl(ctx, "device", "process", "launch", "--device", d.ID, "--payload-url", url, bundleID)
	return err
}

func (m *Manager) screenshotDevice(ctx context.Context, d Device, file string) error {
	out, err := m.devicectl(ctx, "device", "capture", "screenshot", "--device", d.ID, "--destination", file)
	if err != nil && strings.Contains(out, "capture") && strings.Contains(strings.ToLower(out), "unexpected argument") {
		return errors.New("screenshots of a physical iOS device need Xcode 27 or later")
	}
	return err
}

func (m *Manager) recordDevice(d Device, file string) (*exec.Cmd, error) {
	if !m.Tools.HasDevicectl() {
		return nil, errNoDevicectl
	}
	// devicectl saves the mp4 when it gets SIGINT, like simctl recordVideo.
	cmd := exec.Command("xcrun", "devicectl", "device", "capture", "screen-record", "--device", d.ID,
		"--destination", file, "--codec", "h264")
	cmd.Env = m.Tools.XcodeEnv()
	return cmd, nil
}

// AppInstalled reports whether an app with this bundle id is on a physical
// iOS device.
func (m *Manager) AppInstalled(ctx context.Context, d Device, bundleID string) (bool, error) {
	var v struct {
		Result struct {
			Apps []struct {
				BundleIdentifier string `json:"bundleIdentifier"`
			} `json:"apps"`
		} `json:"result"`
	}
	if err := m.devicectlJSON(ctx, &v, "device", "info", "apps", "--device", d.ID, "--bundle-id", bundleID); err != nil {
		return false, err
	}
	for _, a := range v.Result.Apps {
		if a.BundleIdentifier == bundleID {
			return true, nil
		}
	}
	return false, nil
}

// CopyToApp copies a local file or directory into the data container of an
// app on a physical iOS device. dst is relative to the container. With
// replace, a directory on the device keeps only the copied files.
func (m *Manager) CopyToApp(ctx context.Context, d Device, bundleID, src, dst string, replace bool) error {
	_, err := m.devicectl(ctx, "device", "copy", "to", "--device", d.ID,
		"--domain-type", "appDataContainer", "--domain-identifier", bundleID,
		"--source", src, "--destination", dst, "--remove-existing-content", fmt.Sprint(replace))
	return err
}

// CopyFromApp copies a file from the data container of an app on a physical
// iOS device to a local path.
func (m *Manager) CopyFromApp(ctx context.Context, d Device, bundleID, src, dst string) error {
	_, err := m.devicectl(ctx, "device", "copy", "from", "--device", d.ID,
		"--domain-type", "appDataContainer", "--domain-identifier", bundleID,
		"--source", src, "--destination", dst)
	return err
}
