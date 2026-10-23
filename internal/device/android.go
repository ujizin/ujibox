package device

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/proc"
)

var errNoADB = errors.New("adb not found: install the Android SDK platform-tools, or set android_sdk in the global config")

func (m *Manager) adbEnv() []string {
	// adb reads an empty ANDROID_SERIAL as a device name; uji always passes -s.
	return proc.Env(nil, "ANDROID_SERIAL=")
}

func (m *Manager) adb(ctx context.Context, serial string, args ...string) (string, error) {
	if m.Tools.ADB == "" {
		return "", errNoADB
	}
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	return proc.Output(ctx, m.adbEnv(), m.Tools.ADB, args...)
}

func (m *Manager) listAndroid(ctx context.Context) ([]Device, error) {
	if m.Tools.ADB == "" {
		return nil, errNoADB
	}
	out, err := m.adb(ctx, "", "devices", "-l")
	if err != nil {
		return nil, err
	}
	var devs []Device
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{Platform: config.Android, ID: fields[0], Kind: KindPhysical}
		switch fields[1] {
		case "device":
			d.State = StateReady
		case "offline":
			d.State = StateOffline
		case "unauthorized":
			d.State = StateUnauthorized
		default:
			d.State = StateOther
		}
		for _, f := range fields[2:] {
			if v, ok := strings.CutPrefix(f, "model:"); ok {
				d.Model = v
			}
		}
		if strings.HasPrefix(d.ID, "emulator-") {
			d.Kind = KindEmulator
		}
		devs = append(devs, d)
	}

	// Emulator serials follow the console port, so the AVD name is asked each time.
	var wg sync.WaitGroup
	for i := range devs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := &devs[i]
			if d.Kind == KindEmulator {
				if name, err := m.adb(ctx, d.ID, "emu", "avd", "name"); err == nil {
					d.AVD = strings.TrimSpace(strings.Split(strings.ReplaceAll(name, "\r", ""), "\n")[0])
				}
			}
			if d.Ready() {
				if v, err := m.adb(ctx, d.ID, "shell", "getprop", "ro.build.version.release"); err == nil {
					d.OS = strings.TrimSpace(v)
				}
			}
			switch {
			case d.AVD != "":
				d.Name = d.AVD
			case d.Model != "":
				d.Name = strings.ReplaceAll(d.Model, "_", " ")
			default:
				d.Name = d.ID
			}
		}()
	}
	wg.Wait()

	running := map[string]bool{}
	for _, d := range devs {
		if d.AVD != "" {
			running[d.AVD] = true
		}
	}
	for _, avd := range m.listAVDs(ctx) {
		if !running[avd] {
			devs = append(devs, Device{Platform: config.Android, Kind: KindEmulator, Name: avd, AVD: avd, State: StateOff})
		}
	}
	return devs, nil
}

func (m *Manager) listAVDs(ctx context.Context) []string {
	if m.Tools.Emulator != "" {
		if out, err := proc.Output(ctx, nil, m.Tools.Emulator, "-list-avds"); err == nil {
			var avds []string
			for _, l := range strings.Split(out, "\n") {
				l = strings.TrimSpace(l)
				// The emulator prints INFO lines on some versions.
				if l != "" && !strings.Contains(l, " ") && !strings.HasPrefix(l, "INFO") {
					avds = append(avds, l)
				}
			}
			return avds
		}
	}
	dir := os.Getenv("ANDROID_AVD_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".android", "avd")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.ini"))
	var avds []string
	for _, f := range files {
		avds = append(avds, strings.TrimSuffix(filepath.Base(f), ".ini"))
	}
	return avds
}

func (m *Manager) bootAndroid(ctx context.Context, d Device) (Device, error) {
	if d.Ready() {
		return d, nil
	}
	if d.AVD == "" {
		return d, fmt.Errorf("%s is not an emulator; ujibox cannot start it", d.Label())
	}
	if m.Tools.Emulator == "" {
		return d, fmt.Errorf("emulator not found in the Android SDK; cannot boot %s", d.AVD)
	}
	logDir := filepath.Join(paths.Cache(), "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logFile, err := os.Create(filepath.Join(logDir, "emulator-"+d.AVD+".log"))
	if err != nil {
		return d, err
	}
	defer logFile.Close()
	cmd := exec.Command(m.Tools.Emulator, "-avd", d.AVD)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	// Its own session: the emulator outlives uji and ignores the terminal's Ctrl-C.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return d, fmt.Errorf("start emulator %s: %w", d.AVD, err)
	}
	_ = cmd.Process.Release()

	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	for {
		devs, _ := m.listAndroid(ctx)
		for _, x := range devs {
			if x.AVD == d.AVD && x.ID != "" && x.Ready() {
				if v, _ := m.adb(ctx, x.ID, "shell", "getprop", "sys.boot_completed"); strings.TrimSpace(v) == "1" {
					x.Alias = d.Alias
					return x, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return d, fmt.Errorf("%s did not finish booting in time; see %s", d.AVD, paths.Pretty(logFile.Name()))
		case <-time.After(2 * time.Second):
		}
	}
}

// Install failure codes that have a known cause.
var androidInstallHints = map[string]string{
	"INSTALL_FAILED_UPDATE_INCOMPATIBLE":  "the installed app has a different signature; run `uji uninstall` first",
	"INSTALL_FAILED_VERSION_DOWNGRADE":    "the installed app is newer; run `uji uninstall` first",
	"INSTALL_FAILED_NO_MATCHING_ABIS":     "the APK has no native libraries for this device's CPU",
	"INSTALL_FAILED_INSUFFICIENT_STORAGE": "the device is out of storage",
	"INSTALL_FAILED_OLDER_SDK":            "the device's Android version is below the app's minSdk",
}

func (m *Manager) installAndroid(ctx context.Context, d Device, apk string) error {
	out, err := m.adb(ctx, d.ID, "install", "-r", "-d", apk)
	if err == nil && strings.Contains(out, "Success") {
		return nil
	}
	text := out
	if err != nil {
		text = err.Error()
	}
	for code, hint := range androidInstallHints {
		if strings.Contains(text, code) {
			return fmt.Errorf("%s: %s", code, hint)
		}
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("adb install: %s", lastLine(out))
}

func (m *Manager) launchAndroid(ctx context.Context, d Device, pkg string) error {
	out, err := m.adb(ctx, d.ID, "shell", "monkey", "-p", proc.ShellQuote(pkg), "-c", "android.intent.category.LAUNCHER", "1")
	if err != nil {
		return err
	}
	if strings.Contains(out, "No activities found") || strings.Contains(out, "monkey aborted") {
		return fmt.Errorf("%s has no launcher activity on %s (is it installed?)", pkg, d.Label())
	}
	return nil
}

func (m *Manager) uninstallAndroid(ctx context.Context, d Device, pkg string) error {
	out, err := m.adb(ctx, d.ID, "uninstall", pkg)
	if err != nil {
		if strings.Contains(err.Error(), "DELETE_FAILED_INTERNAL_ERROR") || strings.Contains(err.Error(), "Unknown package") {
			return fmt.Errorf("%s is not installed", pkg)
		}
		return err
	}
	if !strings.Contains(out, "Success") {
		return fmt.Errorf("adb uninstall: %s", lastLine(out))
	}
	return nil
}

func (m *Manager) openURLAndroid(ctx context.Context, d Device, url string) error {
	// adb shell joins its arguments into one remote command line: quote the URL
	// for the remote shell, or & ; # and spaces split it.
	if m.Tools.ADB == "" {
		return errNoADB
	}
	out, err := proc.Combined(ctx, m.adbEnv(), m.Tools.ADB, "-s", d.ID, "shell", "am", "start", "-W",
		"-a", "android.intent.action.VIEW", "-d", proc.ShellQuote(url))
	if strings.Contains(out, "unable to resolve Intent") {
		return fmt.Errorf("no app on %s handles this link", d.Label())
	}
	for _, l := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "Error") {
			return fmt.Errorf("%s", t)
		}
	}
	if err != nil {
		return fmt.Errorf("am start: %w: %s", err, lastLine(out))
	}
	return nil
}

func (m *Manager) screenshotAndroid(ctx context.Context, d Device, file string) error {
	if m.Tools.ADB == "" {
		return errNoADB
	}
	cmd := exec.CommandContext(ctx, m.Tools.ADB, "-s", d.ID, "exec-out", "screencap", "-p")
	cmd.Env = m.adbEnv()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("screencap: %w", err)
	}
	// With more than one display, screencap writes a warning before the image.
	i := bytes.Index(out, pngSignature)
	if i < 0 {
		return fmt.Errorf("screencap: no image in the output: %s", lastLine(string(out)))
	}
	return os.WriteFile(file, out[i:], 0o644)
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

func (m *Manager) recordAndroid(d Device, file string, noWindow bool) (*exec.Cmd, error) {
	if m.Tools.Scrcpy == "" {
		return nil, fmt.Errorf("scrcpy not found; install it with `brew install scrcpy`")
	}
	args := []string{"--serial", d.ID, "--record", file, "--window-title", "uji · " + d.Label()}
	if noWindow {
		args = append(args, "--no-window")
	}
	cmd := exec.Command(m.Tools.Scrcpy, args...)
	cmd.Env = proc.Env(nil, "ANDROID_SERIAL=", "ADB="+m.Tools.ADB)
	return cmd, nil
}

// PushDir copies a local directory to a path on the device.
func (m *Manager) PushDir(ctx context.Context, d Device, local, remote string) error {
	if _, err := m.adb(ctx, d.ID, "shell", "mkdir", "-p", proc.ShellQuote(filepath.Dir(remote))); err != nil {
		return err
	}
	_, err := m.adb(ctx, d.ID, "push", local, remote)
	return err
}

// Shell runs a command line in the device shell. Callers quote the values.
func (m *Manager) Shell(ctx context.Context, d Device, cmdline string) (string, error) {
	return m.adb(ctx, d.ID, "shell", cmdline)
}

// Pull copies a file from the device to a local path.
func (m *Manager) Pull(ctx context.Context, d Device, remote, local string) error {
	_, err := m.adb(ctx, d.ID, "pull", remote, local)
	return err
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
