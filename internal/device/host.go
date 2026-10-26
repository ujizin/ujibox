package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/paths"
)

// Desktop and web apps run on this Mac. uji starts them (`uji launch`),
// keeps a record of each process, and captures the window of that process.

// Launched is an app that uji started on this Mac: a desktop app, or the
// Chrome window of a web app.
type Launched struct {
	Platform  string    `json:"platform"`
	PID       int       `json:"pid"`
	Command   string    `json:"command"` // the executable, to tell a reused pid apart
	App       string    `json:"app"`     // display name
	AppID     string    `json:"app_id"`  // store id
	Build     string    `json:"build"`   // short build id
	URL       string    `json:"url,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

func launchedDir() string { return filepath.Join(paths.Cache(), "launched") }

func (l Launched) file() string {
	return filepath.Join(launchedDir(), fmt.Sprintf("%s-%d.json", l.Platform, l.PID))
}

// SaveLaunched records a launched app. Remove the record when the app stops.
func SaveLaunched(l Launched) error {
	if err := os.MkdirAll(launchedDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.file(), data, 0o644)
}

// RemoveLaunched deletes the record of a launched app.
func RemoveLaunched(l Launched) { os.Remove(l.file()) }

// ListLaunched returns the launched apps of the platforms that still run,
// newest first. It deletes the records of apps that stopped.
func ListLaunched(platforms []string) []Launched {
	files, _ := filepath.Glob(filepath.Join(launchedDir(), "*.json"))
	var out []Launched
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var l Launched
		if json.Unmarshal(data, &l) != nil || l.PID <= 0 {
			os.Remove(f)
			continue
		}
		if !running(l) {
			os.Remove(f)
			continue
		}
		for _, p := range platforms {
			if l.Platform == p {
				out = append(out, l)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

// running reports whether the process of a record still runs. A pid that the
// system gave to another program does not count.
func running(l Launched) bool {
	if err := syscall.Kill(l.PID, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(l.PID)).Output()
	if err != nil {
		return false
	}
	comm := strings.TrimSpace(string(out))
	return l.Command == "" || comm == l.Command || filepath.Base(comm) == filepath.Base(l.Command)
}

// Stop ends a launched app and deletes its record.
func (l Launched) Stop() {
	if running(l) {
		_ = syscall.Kill(l.PID, syscall.SIGTERM)
	}
	RemoveLaunched(l)
}

// Device is the launched app as a capture target.
func (l Launched) Device() Device {
	return Device{Platform: l.Platform, Kind: KindWindow, ID: strconv.Itoa(l.PID), Name: l.App, State: StateReady}
}

// Windows returns the launched apps of the platforms as capture targets. It
// is an error when none runs.
func Windows(platforms []string) ([]Device, error) {
	var out []Device
	for _, l := range ListLaunched(platforms) {
		out = append(out, l.Device())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no %s app that uji launched is running; run `uji launch %s` first",
			strings.Join(platforms, " or "), platforms[0])
	}
	return out, nil
}

// LaunchDesktop starts a stored desktop build: the executable of a .app, or
// java -jar for a jar. The app runs on after uji exits; its output goes to
// logFile. It fails when the app exits in its first seconds.
func LaunchDesktop(ctx context.Context, artifact, logFile string, env []string) (pid int, command string, err error) {
	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(artifact, ".app"):
		exe, err := bundleExecutable(ctx, artifact)
		if err != nil {
			return 0, "", err
		}
		cmd = exec.Command(exe)
	case strings.HasSuffix(artifact, ".jar"):
		java := "java"
		if h := os.Getenv("JAVA_HOME"); h != "" {
			java = filepath.Join(h, "bin", "java")
		}
		cmd = exec.Command(java, "-jar", artifact)
	default:
		return 0, "", fmt.Errorf("%s is not a .app or a .jar", artifact)
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return 0, "", err
	}
	log, err := os.Create(logFile)
	if err != nil {
		return 0, "", err
	}
	defer log.Close()
	cmd.Dir = filepath.Dir(artifact)
	cmd.Env = env
	cmd.Stdout = log
	cmd.Stderr = log
	// A new session: Ctrl-C in the terminal does not stop the app.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, "", fmt.Errorf("start %s: %w", filepath.Base(artifact), err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		data, _ := os.ReadFile(logFile)
		msg := lastLine(strings.TrimSpace(string(data)))
		if msg == "" && err != nil {
			msg = err.Error()
		}
		return 0, "", fmt.Errorf("%s stopped right after the start: %s (log: %s)", filepath.Base(artifact), msg, paths.Pretty(logFile))
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
	}
	return cmd.Process.Pid, cmd.Path, nil
}

func bundleExecutable(ctx context.Context, app string) (string, error) {
	plist := filepath.Join(app, "Contents", "Info.plist")
	out, err := exec.CommandContext(ctx, "plutil", "-extract", "CFBundleExecutable", "raw", "-o", "-", plist).Output()
	if err != nil {
		return "", fmt.Errorf("%s has no CFBundleExecutable in Contents/Info.plist", app)
	}
	exe := filepath.Join(app, "Contents", "MacOS", strings.TrimSpace(string(out)))
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("%s has no executable %s", app, filepath.Base(exe))
	}
	return exe, nil
}

// ChromePath returns the Google Chrome executable, or "".
func ChromePath() string {
	home, _ := os.UserHomeDir()
	for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
		p := filepath.Join(dir, "Google Chrome.app", "Contents", "MacOS", "Google Chrome")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ChromeCmd returns the command that opens url in a Chrome app window with
// its own profile in profileDir. A profile of its own makes a separate
// Chrome process, so uji knows the pid of the window.
func ChromeCmd(chrome, url, profileDir string) *exec.Cmd {
	cmd := exec.Command(chrome, "--app="+url, "--user-data-dir="+profileDir,
		"--no-first-run", "--no-default-browser-check")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// window is one entry of CGWindowListCopyWindowInfo.
type window struct {
	Number   int     `json:"kCGWindowNumber"`
	PID      int     `json:"kCGWindowOwnerPID"`
	Layer    int     `json:"kCGWindowLayer"`
	OnScreen bool    `json:"kCGWindowIsOnscreen"`
	Alpha    float64 `json:"kCGWindowAlpha"`
	Bounds   struct {
		X      float64 `json:"X"`
		Y      float64 `json:"Y"`
		Width  float64 `json:"Width"`
		Height float64 `json:"Height"`
	} `json:"kCGWindowBounds"`
}

// windowScript lists the windows of a process with CGWindowListCopyWindowInfo.
// It runs through osascript, so uji needs no cgo for it.
const windowScript = `ObjC.import('CoreGraphics');
function run(argv) {
  const ref = $.CGWindowListCopyWindowInfo($.kCGWindowListOptionAll, $.kCGNullWindowID);
  const list = ObjC.deepUnwrap(ObjC.castRefToObject(ref)) || [];
  const pid = Number(argv[0]);
  return JSON.stringify(list.filter(w => w.kCGWindowOwnerPID === pid));
}`

// findWindow returns the main window of a process: a normal window on
// screen, the largest one.
func findWindow(ctx context.Context, pid int) (window, error) {
	out, err := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", windowScript, strconv.Itoa(pid)).Output()
	if err != nil {
		return window{}, fmt.Errorf("list the windows: %w", err)
	}
	var list []window
	if err := json.Unmarshal(out, &list); err != nil {
		return window{}, fmt.Errorf("list the windows: %w", err)
	}
	return pickWindow(list)
}

func pickWindow(list []window) (window, error) {
	var best window
	found := false
	area := func(w window) float64 { return w.Bounds.Width * w.Bounds.Height }
	for _, w := range list {
		if w.Layer != 0 || !w.OnScreen || w.Alpha == 0 || w.Bounds.Width < 50 || w.Bounds.Height < 50 {
			continue
		}
		if !found || area(w) > area(best) {
			best, found = w, true
		}
	}
	if !found {
		return window{}, errors.New("the app has no window on screen; open it, or bring it back from the Dock")
	}
	return best, nil
}

// screenPermissionHint tells how to give the terminal the permission that
// screencapture needs.
const screenPermissionHint = "give your terminal app the Screen Recording permission in System Settings > Privacy & Security > Screen & System Audio Recording, then restart the terminal"

func checkScreenCapture() error {
	if ok, known := ScreenCaptureAllowed(); known && !ok {
		return errors.New("the terminal cannot capture the screen: " + screenPermissionHint)
	}
	return nil
}

func screenshotWindow(ctx context.Context, d Device, file string) error {
	if err := checkScreenCapture(); err != nil {
		return err
	}
	pid, _ := strconv.Atoi(d.ID)
	w, err := findWindow(ctx, pid)
	if err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "screencapture", "-x", "-o", "-l", strconv.Itoa(w.Number), file).CombinedOutput()
	if err != nil {
		return fmt.Errorf("screencapture: %s; %s", lastLine(strings.TrimSpace(string(out))), screenPermissionHint)
	}
	if st, err := os.Stat(file); err != nil || st.Size() == 0 {
		return errors.New("screencapture wrote no image; " + screenPermissionHint)
	}
	return nil
}

// recordWindow records the window of a launched app with screencapture -v.
// screencapture finishes the movie when it gets SIGINT.
func recordWindow(ctx context.Context, d Device, file string) (*exec.Cmd, error) {
	if err := checkScreenCapture(); err != nil {
		return nil, err
	}
	pid, _ := strconv.Atoi(d.ID)
	w, err := findWindow(ctx, pid)
	if err != nil {
		return nil, err
	}
	return exec.Command("screencapture", "-x", "-v", "-l", strconv.Itoa(w.Number), file), nil
}

// IsHostPlatform reports whether apps of the platform run on this Mac.
func IsHostPlatform(p string) bool { return p == config.Desktop || p == config.Web }
