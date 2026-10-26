package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
)

// RepoFileName is the project file that a repo shares with everyone who
// clones it. It lives in the root of the checkout.
const RepoFileName = ".ujibox.toml"

// RepoFilePath returns the path of the repo file of a checkout.
func RepoFilePath(g gitinfo.Info) string {
	return filepath.Join(g.Root, RepoFileName)
}

// LoadRepoFile reads the .ujibox.toml of the checkout. It returns nil and no
// error when the checkout has none. It does not check that the file is
// complete, because a machine file can add the missing keys.
func LoadRepoFile(g gitinfo.Info) (*Project, error) {
	file := RepoFilePath(g)
	if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	p, err := decodeProjectFile(file)
	if err != nil {
		return nil, err
	}
	p.File = ""
	p.RepoFile = file
	if p.Name == "" || p.Name == ".ujibox" {
		p.Name = slug(filepath.Base(g.MainRoot))
	}
	// The repo file applies to its own checkout; a [match] table means nothing here.
	p.Match = Match{}
	return p, nil
}

// Merge returns the repo project with the values of the machine project on
// top. For every key that the machine file sets, its value wins:
//   - name and default_app;
//   - an app that only the machine file has is added;
//   - in an app, a platform or a device table, a set value replaces the
//     repo value, and env tables merge key by key.
func Merge(repo, local *Project) *Project {
	out := *repo
	out.File = local.File
	out.Match = local.Match
	if local.Name != "" && local.File != "" && local.Name != nameFromFile(local.File) {
		// A name written in the machine file wins; the name from the file name does not.
		out.Name = local.Name
	}
	if local.DefaultApp != "" {
		out.DefaultApp = local.DefaultApp
	}
	out.Apps = map[string]*App{}
	for k, a := range repo.Apps {
		out.Apps[k] = a
	}
	for k, la := range local.Apps {
		ra := out.Apps[k]
		if ra == nil || la == nil {
			if la != nil {
				out.Apps[k] = la
			}
			continue
		}
		out.Apps[k] = mergeApp(ra, la)
	}
	return &out
}

func nameFromFile(file string) string {
	return strings.TrimSuffix(filepath.Base(file), ".toml")
}

func mergeApp(r, l *App) *App {
	out := *r
	if l.Display != "" {
		out.Display = l.Display
	}
	out.Env = mergeEnv(r.Env, l.Env)
	if l.Setup != nil {
		out.Setup = l.Setup
	}
	if l.Clean != nil {
		out.Clean = l.Clean
	}
	if l.Retry != nil {
		out.Retry = l.Retry
	}
	out.Android = mergePlatform(r.Android, l.Android)
	out.IOS = mergePlatform(r.IOS, l.IOS)
	out.Desktop = mergePlatform(r.Desktop, l.Desktop)
	out.Web = mergePlatform(r.Web, l.Web)
	return &out
}

func mergePlatform(r, l *Platform) *Platform {
	if r == nil {
		return l
	}
	if l == nil {
		return r
	}
	out := *r
	if l.ID != "" {
		out.ID = l.ID
	}
	if l.Build != nil {
		out.Build = l.Build
	}
	if l.Artifact != "" {
		out.Artifact = l.Artifact
	}
	out.Env = mergeEnv(r.Env, l.Env)
	if l.Setup != nil {
		out.Setup = l.Setup
	}
	if l.Clean != nil {
		out.Clean = l.Clean
	}
	if l.Retry != nil {
		out.Retry = l.Retry
	}
	out.Device = mergeDevice(r.Device, l.Device)
	return &out
}

func mergeDevice(r, l *DeviceBuild) *DeviceBuild {
	if r == nil {
		return l
	}
	if l == nil {
		return r
	}
	out := *r
	if l.Build != nil {
		out.Build = l.Build
	}
	if l.Artifact != "" {
		out.Artifact = l.Artifact
	}
	out.Env = mergeEnv(r.Env, l.Env)
	return &out
}

func mergeEnv(r, l map[string]string) map[string]string {
	if len(r) == 0 && len(l) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range r {
		out[k] = v
	}
	for k, v := range l {
		out[k] = v
	}
	return out
}

// Sources describes where the config of a project comes from, for output.
func (p *Project) Sources() string {
	switch {
	case p.Detected:
		return "detected (no project file)"
	case p.File != "" && p.RepoFile != "":
		return paths.Pretty(p.RepoFile) + fmt.Sprintf(", overridden by %s", paths.Pretty(p.File))
	case p.RepoFile != "":
		return paths.Pretty(p.RepoFile)
	}
	return paths.Pretty(p.File)
}
