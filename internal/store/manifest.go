// Package store keeps builds outside the project, grouped by app and branch,
// and removes them after the retention period.
package store

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// SchemaVersion is the version of manifest.json. The ujibox app reads this
// format, so a change that breaks readers bumps it.
const SchemaVersion = 1

// Manifest describes one stored build. It is the source of truth: listings
// and the sync index are derived from manifests.
type Manifest struct {
	Schema    int       `json:"schema"`
	ID        string    `json:"id"`
	App       AppRef    `json:"app"`
	Platform  string    `json:"platform"`
	Target    string    `json:"target"` // "emulator" for an APK, "simulator" for an .app
	PackageID string    `json:"package_id"`
	Version   Version   `json:"version"`
	Git       Git       `json:"git"`
	Artifact  Artifact  `json:"artifact"`
	Build     BuildInfo `json:"build"`
	CreatedAt time.Time `json:"created_at"`
	Pinned    bool      `json:"pinned"`

	// Dir is the build directory on disk. It is not stored.
	Dir string `json:"-"`
}

// AppRef names the app a build belongs to.
type AppRef struct {
	ID      string `json:"id"`      // store key, unique across projects
	Name    string `json:"name"`    // display name, for example "Sample App"
	Project string `json:"project"` // project file name
}

// Version is read from the artifact, not from the config.
type Version struct {
	Name string `json:"name,omitempty"` // versionName / CFBundleShortVersionString
	Code string `json:"code,omitempty"` // versionCode / CFBundleVersion
}

// Git is the checkout the build came from.
type Git struct {
	Branch       string `json:"branch,omitempty"`
	Detached     bool   `json:"detached"`
	Worktree     string `json:"worktree"`
	WorktreePath string `json:"worktree_path"`
	SHA          string `json:"sha"`
	Dirty        bool   `json:"dirty"`
	DirtyHash    string `json:"dirty_hash,omitempty"`
	Remote       string `json:"remote,omitempty"`
}

// Artifact is the stored file or bundle, relative to the build directory.
type Artifact struct {
	File   string `json:"file"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// BuildInfo records how the build ran.
type BuildInfo struct {
	Commands   []string `json:"commands"`
	DurationMS int64    `json:"duration_ms"`
	Host       string   `json:"host"`
	Tool       string   `json:"tool"`
}

// Group is the branch the build is listed under, or "@<worktree>" for a
// detached HEAD.
func (m *Manifest) Group() string {
	if m.Git.Branch != "" {
		return m.Git.Branch
	}
	return "@" + m.Git.Worktree
}

// ShortID is enough of the ID to select a build: the random part, because
// builds made in the same millisecond share the time part.
func (m *Manifest) ShortID() string {
	if len(m.ID) >= 26 {
		return strings.ToLower(m.ID[20:26])
	}
	return strings.ToLower(m.ID)
}

// ShortSHA returns 8 characters of the commit.
func (m *Manifest) ShortSHA() string {
	if len(m.Git.SHA) > 8 {
		return m.Git.SHA[:8]
	}
	return m.Git.SHA
}

// ArtifactPath is the absolute path of the stored artifact.
func (m *Manifest) ArtifactPath() string {
	return m.Dir + string(os.PathSeparator) + m.Artifact.File
}

// newULID returns a ULID: 48 bits of milliseconds and 80 random bits in
// Crockford base32. It sorts by time and is safe in any path.
func newULID(t time.Time) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var b [16]byte
	ms := uint64(t.UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	_, _ = rand.Read(b[6:])
	// 128 bits into 26 characters of 5 bits each; the first carries 3 bits.
	out := make([]byte, 26)
	var acc uint64
	bits := 0
	pos := 25
	for i := 15; i >= 0; i-- {
		acc |= uint64(b[i]) << bits
		bits += 8
		for bits >= 5 && pos >= 0 {
			out[pos] = alphabet[acc&31]
			acc >>= 5
			bits -= 5
			pos--
		}
	}
	if pos >= 0 {
		out[pos] = alphabet[acc&31]
	}
	return string(out)
}

func readManifest(file string) (*Manifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeJSON(file string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}
