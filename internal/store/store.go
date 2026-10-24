package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Store is the build store on disk:
//
//	<root>/apps/<app-id>/builds/<build-id>/manifest.json
//	<root>/apps/<app-id>/builds/<build-id>/build.log
//	<root>/apps/<app-id>/builds/<build-id>/<artifact>.apk|.app
//	<root>/tmp/      builds in progress; the same volume, so rename is atomic
//	<root>/locks/
type Store struct {
	Root string
}

// Open returns the store at root. It does not touch the disk.
func Open(root string) *Store { return &Store{Root: root} }

func (s *Store) appDir(appID string) string {
	return filepath.Join(s.Root, "apps", appID, "builds")
}

// TempDir creates a directory for a build in progress.
func (s *Store) TempDir() (string, error) {
	dir := filepath.Join(s.Root, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(dir, "build-*")
}

// Add copies an artifact into the store and writes its manifest. The build
// appears in the store in one rename, so a crash leaves only tmp debris.
func (s *Store) Add(m *Manifest, artifact, buildLog string) (*Manifest, error) {
	now := time.Now().UTC()
	m.Schema = SchemaVersion
	m.ID = newULID(now)
	m.CreatedAt = now

	tmp, err := s.TempDir()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	name := filepath.Base(artifact)
	dst := filepath.Join(tmp, name)
	if err := cloneCopy(artifact, dst); err != nil {
		return nil, fmt.Errorf("copy %s into the store: %w", artifact, err)
	}
	m.Artifact.File = name
	m.Artifact.Size = dirSize(dst)
	if st, err := os.Stat(dst); err == nil && !st.IsDir() {
		m.Artifact.SHA256, _ = fileSHA256(dst)
	}
	if buildLog != "" {
		_ = cloneCopy(buildLog, filepath.Join(tmp, "build.log"))
	}
	if err := writeJSON(filepath.Join(tmp, "manifest.json"), m); err != nil {
		return nil, err
	}
	final := filepath.Join(s.appDir(m.App.ID), m.ID)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, final); err != nil {
		return nil, err
	}
	m.Dir = final
	return m, nil
}

// cloneCopy copies a file or a bundle. On APFS, cp -c makes a clone that
// shares blocks with the source, so a stored copy costs almost no space.
func cloneCopy(src, dst string) error {
	if err := exec.Command("cp", "-cR", src, dst).Run(); err == nil {
		return nil
	}
	// Not APFS, or a partial clone: start over with a plain copy.
	os.RemoveAll(dst)
	if out, err := exec.Command("cp", "-R", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func dirSize(p string) int64 {
	var n int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Filter narrows a listing. Empty fields match everything.
type Filter struct {
	AppID    string
	Platform string
	// Target is "emulator", "simulator" or "device"; see the device package.
	Target string
	Group  string
}

func (f Filter) match(m *Manifest) bool {
	return (f.AppID == "" || m.App.ID == f.AppID) &&
		(f.Platform == "" || m.Platform == f.Platform) &&
		(f.Target == "" || m.Target == f.Target) &&
		(f.Group == "" || m.Group() == f.Group)
}

// List returns the builds that match, newest first. A build directory with a
// missing or broken manifest is skipped: it is half-written or hand-edited.
func (s *Store) List(f Filter) []*Manifest {
	var out []*Manifest
	apps, _ := os.ReadDir(filepath.Join(s.Root, "apps"))
	for _, a := range apps {
		if !a.IsDir() || (f.AppID != "" && a.Name() != f.AppID) {
			continue
		}
		builds, _ := os.ReadDir(s.appDir(a.Name()))
		for _, b := range builds {
			if !b.IsDir() {
				continue
			}
			dir := filepath.Join(s.appDir(a.Name()), b.Name())
			m, err := readManifest(filepath.Join(dir, "manifest.json"))
			if err != nil {
				continue
			}
			m.Dir = dir
			if _, err := os.Stat(m.ArtifactPath()); err != nil {
				continue
			}
			if f.match(m) {
				out = append(out, m)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// ErrNoBuild means no stored build matched.
var ErrNoBuild = errors.New("no stored build")

// Find picks one build. ref is one of:
//   - "" or "last": the newest build in scope
//   - a number N: the Nth newest build in scope (1 is the newest)
//   - a build id, or the short id `uji builds` prints
//   - a commit sha prefix (4+ characters)
//   - a branch name, or @<worktree>: the newest build of that group
//
// scope limits the search for "last" and numbers. Ids, shas and branch names
// search every group of scope's app and platform.
func (s *Store) Find(ref string, scope Filter) (*Manifest, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || ref == "last" {
		list := s.List(scope)
		if len(list) == 0 {
			return nil, ErrNoBuild
		}
		return list[0], nil
	}
	if n, err := strconv.Atoi(ref); err == nil && n > 0 && len(ref) < 4 {
		list := s.List(scope)
		if n > len(list) {
			return nil, fmt.Errorf("%w: only %d builds in this branch", ErrNoBuild, len(list))
		}
		return list[n-1], nil
	}
	wide := Filter{AppID: scope.AppID, Platform: scope.Platform}
	all := s.List(wide)
	lref := strings.ToLower(ref)
	var hits []*Manifest
	for _, m := range all {
		if strings.EqualFold(m.ID, ref) || m.ShortID() == lref {
			return m, nil
		}
	}
	if len(ref) >= 4 && isHex(lref) {
		for _, m := range all {
			if strings.HasPrefix(m.Git.SHA, lref) {
				hits = append(hits, m)
			}
		}
		if len(hits) > 0 {
			return hits[0], nil
		}
	}
	for _, m := range all {
		if m.Group() == ref {
			return m, nil
		}
	}
	return nil, fmt.Errorf("%w matches %q", ErrNoBuild, ref)
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// Use takes a shared lock on a build while it is being installed or synced,
// so eviction skips it. The caller unlocks.
func (s *Store) Use(m *Manifest) *Lock {
	l, err := TryLockShared(filepath.Join(m.Dir, ".lock"))
	if err != nil {
		return nil
	}
	return l
}

// SetPinned pins or unpins a build. A pinned build never expires.
func (s *Store) SetPinned(m *Manifest, pinned bool) error {
	m.Pinned = pinned
	return writeJSON(filepath.Join(m.Dir, "manifest.json"), m)
}

// Remove deletes a build. It fails while another process uses the build.
func (s *Store) Remove(m *Manifest) error {
	l, err := TryLockExclusive(filepath.Join(m.Dir, ".lock"))
	if err != nil {
		return fmt.Errorf("build %s is in use: %w", m.ShortID(), err)
	}
	defer l.Unlock()
	trash := filepath.Join(s.Root, "tmp", "trash-"+m.ID)
	if err := os.MkdirAll(filepath.Dir(trash), 0o755); err != nil {
		return err
	}
	if err := os.Rename(m.Dir, trash); err != nil {
		return err
	}
	return os.RemoveAll(trash)
}

// Policy decides which builds expire.
type Policy struct {
	MaxAge   time.Duration
	KeepLast int
}

// Prune removes expired builds. A build survives if it is pinned, if it is
// one of the newest KeepLast of its app, platform and branch, or if it is
// younger than MaxAge. dryRun lists the builds without removing them.
func (s *Store) Prune(p Policy, now time.Time, dryRun bool) (removed []*Manifest, errs []error) {
	l, err := TryLockExclusive(filepath.Join(s.Root, "locks", "prune.lock"))
	if err != nil {
		return nil, nil // another uji is pruning
	}
	defer l.Unlock()

	seen := map[string]int{}
	for _, m := range s.List(Filter{}) { // newest first
		key := m.App.ID + "\x00" + m.Platform + "\x00" + m.Target + "\x00" + m.Group()
		seen[key]++
		if m.Pinned || seen[key] <= p.KeepLast || now.Sub(m.CreatedAt) < p.MaxAge {
			continue
		}
		if !dryRun {
			if err := s.Remove(m); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		removed = append(removed, m)
	}
	s.cleanTmp(now)
	return removed, errs
}

// cleanTmp removes builds in progress that a crash left behind.
func (s *Store) cleanTmp(now time.Time) {
	entries, _ := os.ReadDir(filepath.Join(s.Root, "tmp"))
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && now.Sub(info.ModTime()) > 24*time.Hour {
			os.RemoveAll(filepath.Join(s.Root, "tmp", e.Name()))
		}
	}
}

// MaybePrune runs Prune at most once a day. It never fails the caller.
func (s *Store) MaybePrune(p Policy) {
	stamp := filepath.Join(s.Root, "locks", "last-prune")
	if st, err := os.Stat(stamp); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
		return
	}
	_ = os.MkdirAll(filepath.Dir(stamp), 0o755)
	_ = os.WriteFile(stamp, nil, 0o644)
	now := time.Now()
	_ = os.Chtimes(stamp, now, now)
	s.Prune(p, now, false)
}
