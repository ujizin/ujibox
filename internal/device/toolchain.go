package device

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/proc"
)

// Tools holds the resolved paths of the external tools. A missing tool is an
// empty string; the commands that need it say so.
type Tools struct {
	ADB          string
	Emulator     string
	AAPT2        string
	Scrcpy       string
	DeveloperDir string
	AndroidSDK   string
}

var (
	toolsOnce sync.Once
	tools     Tools
)

// Resolve finds the tools once per process.
func Resolve(g config.Global) Tools {
	toolsOnce.Do(func() { tools = resolve(g) })
	return tools
}

func resolve(g config.Global) Tools {
	t := Tools{}
	t.AndroidSDK = firstDir(g.AndroidSDK, os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"),
		filepath.Join(os.Getenv("HOME"), "Library", "Android", "sdk"))
	t.ADB = lookPath("adb", filepath.Join(t.AndroidSDK, "platform-tools", "adb"))
	t.Emulator = lookPath("", filepath.Join(t.AndroidSDK, "emulator", "emulator"))
	if t.Emulator == "" {
		t.Emulator = lookPath("emulator")
	}
	t.AAPT2 = lookPath("aapt2", latestBuildTool(t.AndroidSDK, "aapt2"))
	t.Scrcpy = lookPath("scrcpy")
	t.DeveloperDir = developerDir(g.DeveloperDir)
	return t
}

// XcodeEnv is the environment for every Xcode tool. It pins DEVELOPER_DIR,
// because xcode-select often points at the Command Line Tools, which have no simctl.
func (t Tools) XcodeEnv() []string {
	if t.DeveloperDir == "" {
		return nil
	}
	return proc.Env(nil, "DEVELOPER_DIR="+t.DeveloperDir)
}

// HasSimctl reports whether the Xcode toolchain is usable.
func (t Tools) HasSimctl() bool {
	return t.DeveloperDir != "" && fileExists(filepath.Join(t.DeveloperDir, "usr", "bin", "simctl"))
}

func developerDir(configured string) string {
	for _, d := range []string{configured, os.Getenv("DEVELOPER_DIR"), xcodeSelect()} {
		if d != "" && fileExists(filepath.Join(d, "usr", "bin", "simctl")) {
			return d
		}
	}
	apps, _ := filepath.Glob("/Applications/Xcode*.app/Contents/Developer")
	sort.Strings(apps)
	// Prefer the release Xcode over a beta.
	for _, d := range apps {
		if filepath.Base(filepath.Dir(filepath.Dir(d))) == "Xcode.app" && fileExists(filepath.Join(d, "usr", "bin", "simctl")) {
			return d
		}
	}
	for _, d := range apps {
		if fileExists(filepath.Join(d, "usr", "bin", "simctl")) {
			return d
		}
	}
	return ""
}

func xcodeSelect() string {
	out, err := exec.Command("xcode-select", "-p").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func latestBuildTool(sdk, name string) string {
	if sdk == "" {
		return ""
	}
	dirs, _ := filepath.Glob(filepath.Join(sdk, "build-tools", "*", name))
	if len(dirs) == 0 {
		return ""
	}
	sort.Slice(dirs, func(i, j int) bool { return versionLess(dirs[i], dirs[j]) })
	return dirs[len(dirs)-1]
}

// versionLess compares build-tools paths by their numeric version segments.
func versionLess(a, b string) bool {
	va := strings.Split(filepath.Base(filepath.Dir(a)), ".")
	vb := strings.Split(filepath.Base(filepath.Dir(b)), ".")
	for i := 0; i < len(va) && i < len(vb); i++ {
		na, nb := atoi(va[i]), atoi(vb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(va) < len(vb)
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func lookPath(name string, fallbacks ...string) string {
	if name != "" {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	for _, f := range fallbacks {
		if f != "" && fileExists(f) {
			return f
		}
	}
	return ""
}

func firstDir(candidates ...string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ctxOr returns ctx, or a background context when ctx is nil.
func ctxOr(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
