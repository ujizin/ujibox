package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/proc"
)

var errNoSimctl = errors.New("simctl not found: install Xcode, or set developer_dir in the global config")

func (m *Manager) simctl(ctx context.Context, args ...string) (string, error) {
	if !m.Tools.HasSimctl() {
		return "", errNoSimctl
	}
	return proc.Output(ctx, m.Tools.XcodeEnv(), "xcrun", append([]string{"simctl"}, args...)...)
}

func (m *Manager) listIOS(ctx context.Context) ([]Device, error) {
	out, err := m.simctl(ctx, "list", "devices", "--json")
	if err != nil {
		return nil, err
	}
	var v struct {
		Devices map[string][]struct {
			UDID        string `json:"udid"`
			Name        string `json:"name"`
			State       string `json:"state"`
			IsAvailable bool   `json:"isAvailable"`
		} `json:"devices"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return nil, fmt.Errorf("simctl list: %w", err)
	}
	var devs []Device
	for runtime, list := range v.Devices {
		os, ok := iosRuntimeVersion(runtime)
		if !ok {
			continue
		}
		for _, s := range list {
			if !s.IsAvailable {
				continue
			}
			d := Device{Platform: config.IOS, Kind: KindSimulator, ID: s.UDID, Name: s.Name, OS: os}
			switch s.State {
			case "Booted":
				d.State = StateReady
			case "Shutdown":
				d.State = StateOff
			default:
				d.State = StateOther
			}
			devs = append(devs, d)
		}
	}
	return devs, nil
}

// iosRuntimeVersion turns com.apple.CoreSimulator.SimRuntime.iOS-26-5 into
// 26.5. Other runtimes (watchOS, tvOS, visionOS) are not iOS targets.
func iosRuntimeVersion(runtime string) (string, bool) {
	i := strings.LastIndex(runtime, ".iOS-")
	if i < 0 {
		return "", false
	}
	return strings.ReplaceAll(runtime[i+len(".iOS-"):], "-", "."), true
}

func (m *Manager) bootIOS(ctx context.Context, d Device) (Device, error) {
	if !d.Ready() {
		// bootstatus -b boots the simulator and waits until it can take installs.
		ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		defer cancel()
		if _, err := m.simctl(ctx, "bootstatus", d.ID, "-b"); err != nil {
			return d, fmt.Errorf("boot %s: %w", d.Ref(), err)
		}
		d.State = StateReady
	}
	// The window is cosmetic: simctl works headless. Xcode 27 replaced
	// Simulator.app with DeviceHub (com.apple.dt.Devices).
	if exec.Command("open", "-g", "-b", "com.apple.dt.Devices").Run() != nil {
		_ = exec.Command("open", "-g", "-a", "Simulator").Run()
	}
	return d, nil
}

func (m *Manager) installIOS(ctx context.Context, d Device, app string) error {
	if d.Kind == KindPhysical {
		return m.installDevice(ctx, d, app)
	}
	_, err := m.simctl(ctx, "install", d.ID, app)
	return err
}

func (m *Manager) launchIOS(ctx context.Context, d Device, bundleID string) error {
	if d.Kind == KindPhysical {
		return m.launchDevice(ctx, d, bundleID)
	}
	_, err := m.simctl(ctx, "launch", d.ID, bundleID)
	return err
}

func (m *Manager) uninstallIOS(ctx context.Context, d Device, bundleID string) error {
	if d.Kind == KindPhysical {
		return m.uninstallDevice(ctx, d, bundleID)
	}
	_, err := m.simctl(ctx, "uninstall", d.ID, bundleID)
	return err
}

func (m *Manager) openURLIOS(ctx context.Context, d Device, url, bundleID string) error {
	if d.Kind == KindPhysical {
		return m.openURLDevice(ctx, d, url, bundleID)
	}
	// simctl takes an argv, so the URL needs no quoting here.
	_, err := m.simctl(ctx, "openurl", d.ID, url)
	return err
}

func (m *Manager) screenshotIOS(ctx context.Context, d Device, file string) error {
	if d.Kind == KindPhysical {
		return m.screenshotDevice(ctx, d, file)
	}
	_, err := m.simctl(ctx, "io", d.ID, "screenshot", file)
	return err
}

func (m *Manager) recordIOS(d Device, file string) (*exec.Cmd, error) {
	if d.Kind == KindPhysical {
		return m.recordDevice(d, file)
	}
	if !m.Tools.HasSimctl() {
		return nil, errNoSimctl
	}
	cmd := exec.Command("xcrun", "simctl", "io", d.ID, "recordVideo", "--codec=h264", "--mask=black", "--force", file)
	cmd.Env = m.Tools.XcodeEnv()
	return cmd, nil
}

// AppContainer returns the data container of an installed simulator app. A
// physical device has no container path on the Mac; see CopyToApp.
func (m *Manager) AppContainer(ctx context.Context, d Device, bundleID string) (string, error) {
	return m.simctl(ctx, "get_app_container", d.ID, bundleID, "data")
}
