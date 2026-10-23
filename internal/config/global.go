// Package config loads the global ujibox settings and the per-project files.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/ujizin/ujibox/internal/paths"
)

// Global is ~/.config/ujibox/config.toml. Every field is optional.
type Global struct {
	// RetentionDays is how long an unpinned build stays in the store.
	RetentionDays int `toml:"retention_days"`
	// KeepLast is how many builds of each branch and platform survive retention.
	KeepLast int `toml:"keep_last"`
	// DeveloperDir overrides DEVELOPER_DIR and xcode-select for every Xcode tool.
	DeveloperDir string `toml:"developer_dir"`
	// AndroidSDK overrides ANDROID_HOME.
	AndroidSDK string `toml:"android_sdk"`
	// Aliases maps a short name to a device name, AVD name, serial or UDID.
	Aliases map[string]string `toml:"aliases"`
	// SyncPackage is the Android package of the ujibox app that `uji sync` fills.
	SyncPackage string `toml:"sync_package"`
	// SyncBundleID is the bundle id of the ujibox app on iOS simulators.
	SyncBundleID string `toml:"sync_bundle_id"`
}

const (
	defaultRetentionDays = 30
	defaultKeepLast      = 3
	defaultSyncPackage   = "io.ujibox.app"
	defaultSyncBundleID  = "io.ujibox.app"
)

// GlobalPath is the path of the global config file.
func GlobalPath() string {
	return filepath.Join(paths.Config(), "config.toml")
}

// LoadGlobal reads the global config. A missing file gives the defaults. A
// broken file also gives the defaults, plus an error the caller can print.
func LoadGlobal() (Global, error) {
	g := Global{}
	_, err := toml.DecodeFile(GlobalPath(), &g)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	if err != nil {
		err = fmt.Errorf("%s: %w", paths.Pretty(GlobalPath()), err)
	}
	if g.RetentionDays <= 0 {
		g.RetentionDays = defaultRetentionDays
	}
	if g.KeepLast <= 0 {
		g.KeepLast = defaultKeepLast
	}
	if g.SyncPackage == "" {
		g.SyncPackage = defaultSyncPackage
	}
	if g.SyncBundleID == "" {
		g.SyncBundleID = defaultSyncBundleID
	}
	if g.Aliases == nil {
		g.Aliases = map[string]string{}
	}
	return g, err
}

// SetAlias writes one alias into the global config and keeps the other keys.
// It edits the [aliases] table as text, so the comments in the file survive.
func SetAlias(name, target string) error {
	path := GlobalPath()
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	g := Global{}
	if len(data) > 0 {
		if _, err := toml.Decode(string(data), &g); err != nil {
			return fmt.Errorf("%s: %w", paths.Pretty(path), err)
		}
	}
	if g.Aliases == nil {
		g.Aliases = map[string]string{}
	}
	g.Aliases[name] = target

	text := removeTable(string(data), "aliases")
	keys := make([]string, 0, len(g.Aliases))
	for k := range g.Aliases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(strings.TrimRight(text, "\n"))
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString("[aliases]\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = %q\n", quoteKey(k), g.Aliases[k])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, []byte(b.String()))
}

// removeTable drops a [name] table and its keys from TOML text.
func removeTable(text, name string) string {
	var out []string
	skip := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			skip = t == "["+name+"]"
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func quoteKey(k string) string {
	for _, r := range k {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return fmt.Sprintf("%q", k)
		}
	}
	return k
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
