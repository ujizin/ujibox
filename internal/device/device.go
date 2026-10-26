// Package device lists, resolves and drives Android devices, Android
// emulators, iOS simulators and physical iOS devices. It also starts desktop
// and web apps on this Mac and captures their windows.
package device

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ujizin/ujibox/internal/config"
)

// State of a device.
const (
	StateReady        = "ready"        // adb "device", or a booted simulator
	StateOff          = "off"          // an AVD that is not running, or a shut-down simulator
	StateOffline      = "offline"      // adb "offline"
	StateUnauthorized = "unauthorized" // adb "unauthorized", or an iPhone that is not paired
	StateNoDevMode    = "no-dev-mode"  // an iPhone with Developer Mode off
	StateOther        = "other"
)

// Target is what a build runs on. One Android build (an APK) runs on
// emulators and phones; iOS needs a simulator build or a signed device build.
// Desktop and web builds run on this Mac; their target is the platform name.
const (
	TargetEmulator  = "emulator"
	TargetSimulator = "simulator"
	TargetDevice    = "device"
	TargetDesktop   = config.Desktop
	TargetWeb       = config.Web
)

// TargetPlatform returns the platform of a target.
func TargetPlatform(target string) string {
	switch target {
	case TargetEmulator:
		return config.Android
	case TargetDesktop, TargetWeb:
		return target
	}
	return config.IOS
}

// DefaultTarget is the target of a platform when no device says otherwise.
func DefaultTarget(platform string) string {
	switch platform {
	case config.Android:
		return TargetEmulator
	case config.Desktop, config.Web:
		return platform
	}
	return TargetSimulator
}

// Kind of a device.
const (
	KindPhysical  = "device"
	KindEmulator  = "emulator"
	KindSimulator = "simulator"
	// KindWindow is a desktop or web app that uji launched on this Mac.
	KindWindow = "window"
)

// Device is one target. ID is the adb serial or the simulator UDID. For an AVD
// that is not running, ID is empty and AVD holds its name.
type Device struct {
	Platform string `json:"platform"`
	Kind     string `json:"kind"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	AVD      string `json:"avd,omitempty"`
	State    string `json:"state"`
	OS       string `json:"os,omitempty"`
	Model    string `json:"model,omitempty"`
	Alias    string `json:"alias,omitempty"`
	// Wireless is a physical iOS device that devicectl reaches over the network.
	Wireless bool `json:"wireless,omitempty"`
}

// Target is the build target of the device.
func (d Device) Target() string {
	if d.Platform == config.IOS && d.Kind == KindPhysical {
		return TargetDevice
	}
	return DefaultTarget(d.Platform)
}

// Ready reports whether the device accepts installs now.
func (d Device) Ready() bool { return d.State == StateReady }

// Default reports whether a command with no device name uses this device:
// a ready device, but not an iPhone that is only on the same network. That
// iPhone can be someone else's, so it must be named.
func (d Device) Default() bool {
	return d.Ready() && !(d.Kind == KindPhysical && d.Platform == config.IOS && d.Wireless)
}

// Label is the short name used in output: the alias, or the name.
func (d Device) Label() string {
	if d.Alias != "" {
		return d.Alias
	}
	return d.Name
}

// Ref is the device as a user types it to select only this one. A simulator
// name repeats across runtimes, so it carries @<runtime>.
func (d Device) Ref() string {
	if d.Platform == config.IOS && d.OS != "" {
		return d.Name + "@" + d.OS
	}
	return d.Name
}

func (d Device) String() string {
	if d.ID != "" {
		return fmt.Sprintf("%s (%s %s)", d.Label(), d.Platform, d.ID)
	}
	return fmt.Sprintf("%s (%s)", d.Label(), d.Platform)
}

// Manager lists and drives devices.
type Manager struct {
	Tools   Tools
	Aliases map[string]string
}

// NewManager resolves the tools and returns a manager.
func NewManager(g config.Global) *Manager {
	return &Manager{Tools: Resolve(g), Aliases: g.Aliases}
}

// List returns every known device. Errors from one platform do not hide the
// other; they come back in errs.
func (m *Manager) List(ctx context.Context, platforms []string) (devs []Device, errs []error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	add := func(d []Device, err error) {
		mu.Lock()
		defer mu.Unlock()
		devs = append(devs, d...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	for _, p := range platforms {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch p {
			case config.Android:
				add(m.listAndroid(ctx))
			case config.IOS:
				add(m.listIOS(ctx))
				add(m.listIOSDevices(ctx))
			}
		}()
	}
	wg.Wait()
	for i := range devs {
		devs[i].Alias = m.aliasFor(devs[i])
	}
	sort.SliceStable(devs, func(i, j int) bool {
		a, b := devs[i], devs[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		if a.Ready() != b.Ready() {
			return a.Ready()
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.OS > b.OS
	})
	return devs, errs
}

func (m *Manager) aliasFor(d Device) string {
	keys := make([]string, 0, len(m.Aliases))
	for k := range m.Aliases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m.Aliases[k]
		if v == d.ID || v == d.Name || (d.AVD != "" && v == d.AVD) || v == d.Ref() {
			return k
		}
	}
	return ""
}

// AllPlatforms is the list of device platforms. Desktop and web apps run on
// this Mac and are not devices.
var AllPlatforms = []string{config.Android, config.IOS}

// HostPlatforms is the list of platforms whose apps run on this Mac.
var HostPlatforms = []string{config.Desktop, config.Web}

// NotDeviceError is the error of a device command that gets desktop or web.
func NotDeviceError(platform string) error {
	return fmt.Errorf("%s is not a device platform: this command works with android and ios devices; desktop and web apps support build, launch, screenshot and record", platform)
}

// Selection is the parsed form of the selector words of a command.
type Selection struct {
	Platforms []string // empty: every platform
	Names     []string // device references; empty: every ready device
}

// ParseSelectors splits words into platform words and device names. The
// flags --android and --ios add platform words. "desktop" and "web" are
// platform words too; "all" means every device platform.
func ParseSelectors(words []string, android, ios bool) Selection {
	s := Selection{}
	add := func(p string) {
		for _, x := range s.Platforms {
			if x == p {
				return
			}
		}
		s.Platforms = append(s.Platforms, p)
	}
	if android {
		add(config.Android)
	}
	if ios {
		add(config.IOS)
	}
	for _, w := range words {
		switch strings.ToLower(w) {
		case "android":
			add(config.Android)
		case "ios":
			add(config.IOS)
		case "desktop":
			add(config.Desktop)
		case "web":
			add(config.Web)
		case "all":
			add(config.Android)
			add(config.IOS)
		default:
			s.Names = append(s.Names, w)
		}
	}
	return s
}

// PlatformsOr returns the selected platforms, or def when none were named.
func (s Selection) PlatformsOr(def []string) []string {
	if len(s.Platforms) == 0 {
		return def
	}
	return s.Platforms
}

// Split returns the selection without the desktop and web words, and those
// words.
func (s Selection) Split() (devices Selection, host []string) {
	devices.Names = s.Names
	for _, p := range s.Platforms {
		if config.IsDevicePlatform(p) {
			devices.Platforms = append(devices.Platforms, p)
		} else {
			host = append(host, p)
		}
	}
	return devices, host
}

// devicePlatforms keeps the device platforms of a list.
func devicePlatforms(platforms []string) []string {
	var out []string
	for _, p := range platforms {
		if config.IsDevicePlatform(p) {
			out = append(out, p)
		}
	}
	return out
}

// Resolve turns a selection into devices.
//   - With no names, it returns every ready device of the selected platforms
//     (see Device.Default). It never boots anything in that case.
//   - A name matches, in order: an alias, an exact serial or UDID, an exact
//     name or AVD name (with @<os> for a simulator), then a unique prefix.
//   - A name that matches several devices is an error that lists them, unless
//     exactly one of them is ready.
//   - A desktop or web word is an error: those apps are not on a device. The
//     desktop and web platforms of defPlatforms are left out.
func (m *Manager) Resolve(ctx context.Context, sel Selection, defPlatforms []string) ([]Device, error) {
	if _, host := sel.Split(); len(host) > 0 {
		return nil, NotDeviceError(host[0])
	}
	platforms := sel.PlatformsOr(devicePlatforms(defPlatforms))
	if len(platforms) == 0 {
		return nil, NotDeviceError(strings.Join(defPlatforms, " and "))
	}
	devs, errs := m.List(ctx, platforms)
	if len(sel.Names) == 0 {
		var out []Device
		for _, d := range devs {
			if d.Default() {
				out = append(out, d)
			}
		}
		if len(out) == 0 {
			return nil, noDevicesError(platforms, devs, errs)
		}
		return out, nil
	}

	var out []Device
	seen := map[string]bool{}
	for _, name := range sel.Names {
		d, err := match(name, devs, m.Aliases)
		if err != nil {
			if len(errs) > 0 {
				return nil, fmt.Errorf("%w\n(also: %v)", err, joinErrs(errs))
			}
			return nil, err
		}
		key := d.Platform + "/" + d.ID + "/" + d.AVD + "/" + d.Name
		if !seen[key] {
			seen[key] = true
			out = append(out, d)
		}
	}
	return out, nil
}

func match(name string, devs []Device, aliases map[string]string) (Device, error) {
	want := name
	if v, ok := aliases[name]; ok {
		want = v
	}
	lw := strings.ToLower(want)
	tiers := []func(Device) bool{
		func(d Device) bool { return d.ID != "" && d.ID == want },
		func(d Device) bool {
			return strings.EqualFold(d.Ref(), want) || strings.EqualFold(d.Name, want) || (d.AVD != "" && strings.EqualFold(d.AVD, want))
		},
		func(d Device) bool {
			return strings.HasPrefix(strings.ToLower(d.Name), lw) || (d.AVD != "" && strings.HasPrefix(strings.ToLower(d.AVD), lw)) ||
				(d.ID != "" && strings.HasPrefix(strings.ToLower(d.ID), lw))
		},
	}
	for _, t := range tiers {
		var hits []Device
		for _, d := range devs {
			if t(d) {
				hits = append(hits, d)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		}
		var ready []Device
		for _, d := range hits {
			if d.Ready() {
				ready = append(ready, d)
			}
		}
		if len(ready) == 1 {
			return ready[0], nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%q matches %d devices; use one of these names:", name, len(hits))
		for _, d := range hits {
			fmt.Fprintf(&b, "\n  %-32s %s %s", d.Ref(), d.State, d.ID)
		}
		return Device{}, fmt.Errorf("%s", b.String())
	}
	return Device{}, fmt.Errorf("no device matches %q; run `uji devices --all` to see the names", name)
}

func noDevicesError(platforms []string, devs []Device, errs []error) error {
	var b strings.Builder
	fmt.Fprintf(&b, "no ready %s device", strings.Join(platforms, " or "))
	var notReady []string
	for _, d := range devs {
		switch d.State {
		case StateOffline, StateUnauthorized, StateNoDevMode, StateOther:
			notReady = append(notReady, fmt.Sprintf("%s is %s", d.Label(), d.State))
		}
	}
	if len(notReady) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(notReady, "; "))
	}
	b.WriteString("; start an emulator or simulator, or name one to boot it, e.g. `uji boot <name>`")
	if len(errs) > 0 {
		fmt.Fprintf(&b, "\n%v", joinErrs(errs))
	}
	return fmt.Errorf("%s", b.String())
}

func joinErrs(errs []error) string {
	parts := make([]string, len(errs))
	for i, e := range errs {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "\n")
}

// EnsureReady boots each device that is off. Devices that are offline or
// unauthorized come back as errors, because ujibox cannot fix them.
func (m *Manager) EnsureReady(ctx context.Context, devs []Device, log func(string)) ([]Device, error) {
	out := make([]Device, len(devs))
	for i, d := range devs {
		switch d.State {
		case StateReady:
			out[i] = d
		case StateOff:
			log(fmt.Sprintf("booting %s…", d.Label()))
			b, err := m.Boot(ctx, d)
			if err != nil {
				return nil, err
			}
			out[i] = b
		default:
			return nil, notReadyError(d)
		}
	}
	return out, nil
}

// notReadyError tells what to do with a device that uji cannot use now.
func notReadyError(d Device) error {
	ios := d.Platform == config.IOS
	switch {
	case d.State == StateUnauthorized && ios:
		return fmt.Errorf("%s is not paired: connect it with a cable, unlock it and tap Trust", d.Label())
	case d.State == StateUnauthorized:
		return fmt.Errorf("%s is unauthorized: unlock it and accept the USB debugging prompt", d.Label())
	case d.State == StateNoDevMode:
		return fmt.Errorf("%s has Developer Mode off: turn it on in Settings > Privacy & Security > Developer Mode", d.Label())
	case ios:
		return fmt.Errorf("%s is %s: unlock it, and connect it with a cable or join the same network as this Mac", d.Label(), d.State)
	}
	return fmt.Errorf("%s is %s: reconnect it, or run `adb reconnect`", d.Label(), d.State)
}

// Boot starts an AVD or a simulator and waits until it is ready.
func (m *Manager) Boot(ctx context.Context, d Device) (Device, error) {
	if d.Kind == KindPhysical {
		return d, fmt.Errorf("%s is a physical device; uji cannot boot it", d.Label())
	}
	switch d.Platform {
	case config.Android:
		return m.bootAndroid(ctx, d)
	case config.IOS:
		return m.bootIOS(ctx, d)
	}
	return d, fmt.Errorf("unknown platform %q", d.Platform)
}

// Shutdown stops an emulator or a simulator. A physical device is an error.
func (m *Manager) Shutdown(ctx context.Context, d Device) error {
	switch d.Platform {
	case config.Android:
		if d.Kind != KindEmulator {
			return fmt.Errorf("%s is a physical device", d.Label())
		}
		_, err := m.adb(ctx, d.ID, "emu", "kill")
		return err
	case config.IOS:
		if d.Kind == KindPhysical {
			return fmt.Errorf("%s is a physical device", d.Label())
		}
		_, err := m.simctl(ctx, "shutdown", d.ID)
		return err
	}
	return nil
}
