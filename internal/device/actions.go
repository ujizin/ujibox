package device

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ujizin/ujibox/internal/config"
)

// Install puts an artifact (APK or simulator .app) on a device.
func (m *Manager) Install(ctx context.Context, d Device, artifact string) error {
	switch d.Platform {
	case config.Android:
		return m.installAndroid(ctx, d, artifact)
	case config.IOS:
		return m.installIOS(ctx, d, artifact)
	}
	return fmt.Errorf("unknown platform %q", d.Platform)
}

// Launch starts an installed app by its application id or bundle id.
func (m *Manager) Launch(ctx context.Context, d Device, id string) error {
	switch d.Platform {
	case config.Android:
		return m.launchAndroid(ctx, d, id)
	case config.IOS:
		return m.launchIOS(ctx, d, id)
	}
	return fmt.Errorf("unknown platform %q", d.Platform)
}

// Uninstall removes an app by its application id or bundle id.
func (m *Manager) Uninstall(ctx context.Context, d Device, id string) error {
	switch d.Platform {
	case config.Android:
		return m.uninstallAndroid(ctx, d, id)
	case config.IOS:
		return m.uninstallIOS(ctx, d, id)
	}
	return fmt.Errorf("unknown platform %q", d.Platform)
}

// OpenURL opens a deeplink or a web link on a device.
func (m *Manager) OpenURL(ctx context.Context, d Device, url string) error {
	switch d.Platform {
	case config.Android:
		return m.openURLAndroid(ctx, d, url)
	case config.IOS:
		return m.openURLIOS(ctx, d, url)
	}
	return fmt.Errorf("unknown platform %q", d.Platform)
}

// Screenshot writes a PNG of the device screen.
func (m *Manager) Screenshot(ctx context.Context, d Device, file string) error {
	switch d.Platform {
	case config.Android:
		return m.screenshotAndroid(ctx, d, file)
	case config.IOS:
		return m.screenshotIOS(ctx, d, file)
	}
	return fmt.Errorf("unknown platform %q", d.Platform)
}

// RecordCmd returns the command that records the screen into file. The caller
// starts it, and stops it with SIGINT so the tool can finalize the mp4.
func (m *Manager) RecordCmd(d Device, file string, noWindow bool) (*exec.Cmd, error) {
	switch d.Platform {
	case config.Android:
		return m.recordAndroid(d, file, noWindow)
	case config.IOS:
		return m.recordIOS(d, file)
	}
	return nil, fmt.Errorf("unknown platform %q", d.Platform)
}
