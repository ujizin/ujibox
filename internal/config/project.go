package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
)

// Platform names.
const (
	Android = "android"
	IOS     = "ios"
)

// Project is one file in ~/.config/ujibox/projects. It tells ujibox how to
// build the apps of a repo. It lives on the user's machine, never in the repo.
type Project struct {
	Name       string          `toml:"name"`
	Match      Match           `toml:"match"`
	DefaultApp string          `toml:"default_app,omitempty"`
	Apps       map[string]*App `toml:"apps"`

	// File is the path the project came from. It is empty for a detected project.
	File string `toml:"-"`
	// Detected is true when no file matched and ujibox guessed the config.
	Detected bool `toml:"-"`
}

// Match selects the checkouts a project applies to.
type Match struct {
	// Remote is a normalized origin URL, for example github.com/org/repo.
	Remote string `toml:"remote,omitempty"`
	// Paths are globs of checkout roots. They may start with ~.
	Paths []string `toml:"paths,omitempty"`
}

// App is one installable app of a project.
type App struct {
	Display string            `toml:"display,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
	Setup   []Step            `toml:"setup,omitempty"`
	Clean   []Step            `toml:"clean,omitempty"`
	Retry   []Retry           `toml:"retry,omitempty"`
	Android *Platform         `toml:"android,omitempty"`
	IOS     *Platform         `toml:"ios,omitempty"`

	// Key is the table name under [apps]. ujibox fills it in.
	Key string `toml:"-"`
}

// Platform tells ujibox how to build one platform of an app.
type Platform struct {
	// ID is the Android application id or the iOS bundle id. ujibox reads the
	// real value from the artifact after a build; this one serves launch,
	// uninstall and builds that are not stored yet.
	ID string `toml:"id,omitempty"`
	// Build is the list of shell commands that produce the artifact.
	Build []string `toml:"build"`
	// Artifact is a glob, relative to the checkout root, that finds the APK or
	// the .app after the build. "**" matches any number of directories.
	Artifact string            `toml:"artifact"`
	Env      map[string]string `toml:"env,omitempty"`
	Setup    []Step            `toml:"setup,omitempty"`
	Clean    []Step            `toml:"clean,omitempty"`
	Retry    []Retry           `toml:"retry,omitempty"`
	// Device builds the iOS app for physical devices. It is iOS only.
	Device *DeviceBuild `toml:"device,omitempty"`

	// gradleRoot is the directory of the Gradle settings file, relative to
	// the checkout root. Only detection sets it.
	gradleRoot string
}

// DeviceBuild is the build of an iOS app for physical devices. It needs code
// signing. The setup, clean and retry steps and the id come from the iOS
// platform; Env adds to its env.
type DeviceBuild struct {
	Build    []string          `toml:"build"`
	Artifact string            `toml:"artifact"`
	Env      map[string]string `toml:"env,omitempty"`
}

// ForDevice returns the platform with the device build in place of the
// simulator build.
func (p *Platform) ForDevice() (*Platform, error) {
	if p.Device == nil || len(p.Device.Build) == 0 {
		return nil, errors.New("this app has no iOS device build; add [apps.<app>.ios.device] to the project file (`uji config edit`)")
	}
	out := *p
	out.Build = p.Device.Build
	out.Artifact = p.Device.Artifact
	out.Env = map[string]string{}
	for k, v := range p.Env {
		out.Env[k] = v
	}
	for k, v := range p.Device.Env {
		out.Env[k] = v
	}
	out.Device = nil
	return &out, nil
}

// Step is one shell command. If Creates is set and that path exists, the step
// does not run again. That makes a one-time worktree setup cheap to repeat.
type Step struct {
	Run     string `toml:"run"`
	Creates string `toml:"creates,omitempty"`
}

// Retry runs Then and retries the build once when the build log matches Match.
type Retry struct {
	Match string `toml:"match"`
	Then  string `toml:"then"`
}

// Name returns the display name of the app.
func (a *App) Name() string {
	if a.Display != "" {
		return a.Display
	}
	return a.Key
}

// Platform returns the config of one platform, or nil.
func (a *App) Platform(p string) *Platform {
	switch p {
	case Android:
		return a.Android
	case IOS:
		return a.IOS
	}
	return nil
}

// Platforms lists the platforms the app can build, Android first.
func (a *App) Platforms() []string {
	var out []string
	if a.Android != nil {
		out = append(out, Android)
	}
	if a.IOS != nil {
		out = append(out, IOS)
	}
	return out
}

// StoreID is the key of the app in the build store. It is unique across
// projects, so two projects with an app called "sample" do not mix.
func (p *Project) StoreID(a *App) string {
	if a.Key == p.Name {
		return a.Key
	}
	return p.Name + "." + a.Key
}

// AppKeys lists the app keys in a stable order.
func (p *Project) AppKeys() []string {
	keys := make([]string, 0, len(p.Apps))
	for k := range p.Apps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// App picks an app by key. With an empty key it picks the default app, or the
// only app.
func (p *Project) App(key string) (*App, error) {
	if key == "" {
		key = p.DefaultApp
	}
	if key == "" {
		if len(p.Apps) == 1 {
			for _, a := range p.Apps {
				return a, nil
			}
		}
		return nil, fmt.Errorf("project %q has %d apps (%s); pick one with --app, or set default_app in %s",
			p.Name, len(p.Apps), strings.Join(p.AppKeys(), ", "), p.where())
	}
	if a, ok := p.Apps[key]; ok {
		return a, nil
	}
	for _, a := range p.Apps {
		if strings.EqualFold(a.Display, key) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("project %q has no app %q; known apps: %s", p.Name, key, strings.Join(p.AppKeys(), ", "))
}

func (p *Project) where() string {
	if p.File == "" {
		return "a project file (run `uji config project`)"
	}
	return paths.Pretty(p.File)
}

// LoadProjects reads every project file. A broken file is skipped and
// reported in errs, so one bad file does not stop the others.
func LoadProjects() (projects []*Project, errs []error) {
	files, _ := filepath.Glob(filepath.Join(paths.Projects(), "*.toml"))
	sort.Strings(files)
	for _, f := range files {
		p, err := LoadProjectFile(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		projects = append(projects, p)
	}
	return projects, errs
}

// LoadProjectFile reads one project file.
func LoadProjectFile(file string) (*Project, error) {
	p := &Project{}
	md, err := toml.DecodeFile(file, p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", paths.Pretty(file), err)
	}
	if und := md.Undecoded(); len(und) > 0 {
		keys := make([]string, len(und))
		for i, k := range und {
			keys[i] = k.String()
		}
		return nil, fmt.Errorf("%s: unknown keys: %s", paths.Pretty(file), strings.Join(keys, ", "))
	}
	p.File = file
	if p.Name == "" {
		p.Name = strings.TrimSuffix(filepath.Base(file), ".toml")
	}
	if err := p.normalize(); err != nil {
		return nil, fmt.Errorf("%s: %w", paths.Pretty(file), err)
	}
	return p, nil
}

func (p *Project) normalize() error {
	if len(p.Apps) == 0 {
		return fmt.Errorf("no [apps.<name>] table")
	}
	for k, a := range p.Apps {
		if a == nil {
			return fmt.Errorf("app %q is empty", k)
		}
		a.Key = k
		if a.Android == nil && a.IOS == nil {
			return fmt.Errorf("app %q has neither [apps.%s.android] nor [apps.%s.ios]", k, k, k)
		}
		for _, name := range a.Platforms() {
			pl := a.Platform(name)
			if len(pl.Build) == 0 {
				return fmt.Errorf("apps.%s.%s.build is empty", k, name)
			}
			if pl.Artifact == "" {
				return fmt.Errorf("apps.%s.%s.artifact is empty", k, name)
			}
		}
	}
	if p.DefaultApp != "" {
		if _, ok := p.Apps[p.DefaultApp]; !ok {
			return fmt.Errorf("default_app %q is not under [apps]", p.DefaultApp)
		}
	}
	return nil
}

// Matches reports whether the project applies to a checkout.
func (p *Project) Matches(g gitinfo.Info) bool {
	if p.Match.Remote != "" && g.Remote != "" && gitinfo.NormalizeRemote(p.Match.Remote) == g.Remote {
		return true
	}
	for _, pat := range p.Match.Paths {
		pat = paths.Expand(pat)
		for _, root := range []string{g.Root, g.MainRoot} {
			if ok, _ := filepath.Match(pat, root); ok {
				return true
			}
			if strings.TrimSuffix(pat, "/") == root {
				return true
			}
		}
	}
	return false
}

// Find returns the project for a checkout: the first file that matches, or a
// detected project. It returns nil when nothing matches and detection finds
// nothing to build.
func Find(g gitinfo.Info) (*Project, []error) {
	projects, errs := LoadProjects()
	for _, p := range projects {
		if p.Matches(g) {
			return p, errs
		}
	}
	if p := Detect(g); p != nil {
		return p, errs
	}
	return nil, errs
}

// Vars are the values a step or build command can use as {name}. Each one is
// also exported to the command as UJI_<NAME>.
type Vars struct {
	Root        string
	MainRoot    string
	DerivedData string
	Branch      string
	Worktree    string
	App         string
	Platform    string
}

// DerivedDataDir is the per-checkout DerivedData directory in the cache.
func DerivedDataDir(root string) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(paths.Cache(), "derived-data", filepath.Base(root)+"-"+hex.EncodeToString(sum[:4]))
}

// NewVars builds the variables of one checkout, app and platform.
func NewVars(g gitinfo.Info, app *App, platform string) Vars {
	return Vars{
		Root:        g.Root,
		MainRoot:    g.MainRoot,
		DerivedData: DerivedDataDir(g.Root),
		Branch:      g.Group(),
		Worktree:    g.Worktree,
		App:         app.Key,
		Platform:    platform,
	}
}

func (v Vars) pairs() [][2]string {
	return [][2]string{
		{"root", v.Root},
		{"main_root", v.MainRoot},
		{"derived_data", v.DerivedData},
		{"branch", v.Branch},
		{"worktree", v.Worktree},
		{"app", v.App},
		{"platform", v.Platform},
	}
}

// Expand replaces {name} with its value. An unknown {name} stays as it is, so
// shell syntax like ${HOME} or awk braces passes through.
func (v Vars) Expand(s string) string {
	for _, kv := range v.pairs() {
		s = strings.ReplaceAll(s, "{"+kv[0]+"}", kv[1])
	}
	return s
}

// Env returns the variables as UJI_ROOT=... pairs.
func (v Vars) Env() []string {
	var out []string
	for _, kv := range v.pairs() {
		out = append(out, "UJI_"+strings.ToUpper(kv[0])+"="+kv[1])
	}
	return out
}

// Exists reports whether a path exists.
func Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
