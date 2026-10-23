// Package paths resolves where ujibox keeps its data, caches and configuration.
package paths

import (
	"os"
	"path/filepath"
	"strings"
)

// Data is the build store. It holds builds that must survive cache sweeps.
// UJIBOX_HOME overrides it.
func Data() string {
	if v := os.Getenv("UJIBOX_HOME"); v != "" {
		return Expand(v)
	}
	return filepath.Join(home(), "Library", "Application Support", "ujibox")
}

// Cache holds data that ujibox can rebuild: DerivedData, unpacked bundles, locks.
// It stays out of /tmp, because macOS sweeps /tmp and empties SPM checkouts.
func Cache() string {
	if v := os.Getenv("UJIBOX_CACHE"); v != "" {
		return Expand(v)
	}
	return filepath.Join(home(), "Library", "Caches", "ujibox")
}

// Config holds the global config and the per-project files.
func Config() string {
	if v := os.Getenv("UJIBOX_CONFIG"); v != "" {
		return Expand(v)
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "ujibox")
	}
	return filepath.Join(home(), ".config", "ujibox")
}

// Projects is the directory of per-project config files.
func Projects() string {
	return filepath.Join(Config(), "projects")
}

// Expand replaces a leading ~ with the home directory.
func Expand(p string) string {
	if p == "~" {
		return home()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home(), p[2:])
	}
	return p
}

// Pretty replaces the home directory prefix with ~ for display.
func Pretty(p string) string {
	h := home()
	if h != "" && strings.HasPrefix(p, h+string(filepath.Separator)) {
		return "~" + p[len(h):]
	}
	return p
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
