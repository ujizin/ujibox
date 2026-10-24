// Package syncer copies stored builds into the ujibox app on a device.
//
// The layout on the device, under the app's own storage, is:
//
//	ujibox/index.json                 written last; the app reads only this
//	ujibox/builds/<id>/manifest.json
//	ujibox/builds/<id>/<artifact>     .apk on Android, .app on a simulator
//
// Android: the app's external files dir, /sdcard/Android/data/<pkg>/files.
// iOS simulator: the app's data container, <container>/Documents.
// Physical iOS device: the same Documents folder, through devicectl. It gets
// only the manifests: the phone cannot install the .app, and the install
// command runs on the Mac with the stored build.
package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/proc"
	"github.com/ujizin/ujibox/internal/store"
)

// IndexVersion is the version of index.json. The ujibox app reads it.
const IndexVersion = 1

// Index is ujibox/index.json on the device.
type Index struct {
	Schema    int       `json:"schema"`
	UpdatedAt time.Time `json:"updated_at"`
	Host      string    `json:"host"`
	Platform  string    `json:"platform"`
	Apps      []App     `json:"apps"`
}

// App groups builds by branch, newest branch first.
type App struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Branches []Branch `json:"branches"`
}

// Branch lists builds newest first. Name is "@<worktree>" for a detached HEAD.
type Branch struct {
	Name     string  `json:"name"`
	Detached bool    `json:"detached"`
	Builds   []Build `json:"builds"`
}

// Build is one row in the app. It has what a list row shows, so the app
// does not open each manifest.
type Build struct {
	ID        string        `json:"id"`
	PackageID string        `json:"package_id"`
	Version   store.Version `json:"version"`
	SHA       string        `json:"sha"`
	CommitURL string        `json:"commit_url,omitempty"` // the web page of the commit, if the host is known
	Dirty     bool          `json:"dirty"`
	Pinned    bool          `json:"pinned"`
	CreatedAt time.Time     `json:"created_at"`
	Size      int64         `json:"size"`
	Manifest  string        `json:"manifest"` // path relative to ujibox/
	Install   Install       `json:"install"`
}

// Install tells the app how to install a build.
//   - kind "apk": File is an APK the app installs with PackageInstaller.
//   - kind "command": iOS apps cannot install other apps, on a device or a
//     simulator. Command is the line to run in a Mac terminal; the app copies
//     it to the pasteboard.
type Install struct {
	Kind    string `json:"kind"`
	File    string `json:"file,omitempty"`
	Command string `json:"command,omitempty"`
}

// ErrNoApp means the ujibox app is not installed on the device.
var ErrNoApp = errors.New("the ujibox app is not installed")

// Result of one device sync.
type Result struct {
	Pushed  int
	Removed int
	Kept    int
}

// Target is where the files go: adb on Android, the file system on a simulator.
type target interface {
	readIndex(ctx context.Context) (*Index, error)
	push(ctx context.Context, m *store.Manifest) error
	remove(ctx context.Context, id string) error
	writeIndex(ctx context.Context, data []byte) error
	installFor(m *store.Manifest) Install
}

// Sync makes the device hold exactly the given builds of its platform.
func Sync(ctx context.Context, mgr *device.Manager, g config.Global, d device.Device, builds []*store.Manifest) (Result, error) {
	var t target
	switch d.Platform {
	case config.Android:
		out, err := mgr.Shell(ctx, d, "pm path "+proc.ShellQuote(g.SyncPackage))
		if err != nil || !strings.HasPrefix(strings.TrimSpace(out), "package:") {
			return Result{}, fmt.Errorf("%w on %s (%s)", ErrNoApp, d.Label(), g.SyncPackage)
		}
		t = &androidTarget{mgr: mgr, dev: d, root: "/sdcard/Android/data/" + g.SyncPackage + "/files/ujibox"}
	case config.IOS:
		if d.Kind == device.KindPhysical {
			ok, err := mgr.AppInstalled(ctx, d, g.SyncBundleID)
			if err != nil {
				return Result{}, err
			}
			if !ok {
				return Result{}, fmt.Errorf("%w on %s (%s)", ErrNoApp, d.Label(), g.SyncBundleID)
			}
			t = &phoneTarget{mgr: mgr, dev: d, bundle: g.SyncBundleID}
			break
		}
		dir, err := mgr.AppContainer(ctx, d, g.SyncBundleID)
		if err != nil || dir == "" {
			return Result{}, fmt.Errorf("%w on %s (%s)", ErrNoApp, d.Label(), g.SyncBundleID)
		}
		t = &simTarget{dev: d, root: filepath.Join(dir, "Documents", "ujibox")}
	default:
		return Result{}, fmt.Errorf("unknown platform %q", d.Platform)
	}

	old, _ := t.readIndex(ctx)
	have := map[string]bool{}
	if old != nil {
		for _, a := range old.Apps {
			for _, b := range a.Branches {
				for _, x := range b.Builds {
					have[x.ID] = true
				}
			}
		}
	}
	want := map[string]bool{}
	res := Result{}
	for _, m := range builds {
		want[m.ID] = true
		if have[m.ID] {
			res.Kept++
			continue
		}
		if err := t.push(ctx, m); err != nil {
			return res, fmt.Errorf("copy build %s: %w", m.ShortID(), err)
		}
		res.Pushed++
	}

	// The index goes first, so the app never lists a build that is being removed.
	host, _ := os.Hostname()
	idx := BuildIndex(builds, d.Platform, host, t.installFor)
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return res, err
	}
	if err := t.writeIndex(ctx, data); err != nil {
		return res, fmt.Errorf("write index: %w", err)
	}
	for id := range have {
		if !want[id] {
			if err := t.remove(ctx, id); err == nil {
				res.Removed++
			}
		}
	}
	return res, nil
}

// BuildIndex groups builds into apps and branches, newest first.
func BuildIndex(builds []*store.Manifest, platform, host string, install func(*store.Manifest) Install) Index {
	idx := Index{Schema: IndexVersion, UpdatedAt: time.Now().UTC(), Host: host, Platform: platform, Apps: []App{}}
	appPos := map[string]int{}
	branchPos := map[string]int{}
	for _, m := range builds { // newest first
		ai, ok := appPos[m.App.ID]
		if !ok {
			ai = len(idx.Apps)
			appPos[m.App.ID] = ai
			idx.Apps = append(idx.Apps, App{ID: m.App.ID, Name: m.App.Name})
		}
		bk := m.App.ID + "\x00" + m.Group()
		bi, ok := branchPos[bk]
		if !ok {
			bi = len(idx.Apps[ai].Branches)
			branchPos[bk] = bi
			idx.Apps[ai].Branches = append(idx.Apps[ai].Branches, Branch{Name: m.Group(), Detached: m.Git.Detached})
		}
		idx.Apps[ai].Branches[bi].Builds = append(idx.Apps[ai].Branches[bi].Builds, Build{
			ID: m.ID, PackageID: m.PackageID, Version: m.Version, SHA: m.Git.SHA,
			CommitURL: gitinfo.CommitURL(m.Git.Remote, m.Git.SHA), Dirty: m.Git.Dirty,
			Pinned: m.Pinned, CreatedAt: m.CreatedAt, Size: m.Artifact.Size,
			Manifest: path.Join("builds", m.ID, "manifest.json"),
			Install:  install(m),
		})
	}
	return idx
}

type androidTarget struct {
	mgr  *device.Manager
	dev  device.Device
	root string
}

func (t *androidTarget) readIndex(ctx context.Context) (*Index, error) {
	out, err := t.mgr.Shell(ctx, t.dev, "cat "+proc.ShellQuote(t.root+"/index.json")+" 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil, err
	}
	idx := &Index{}
	if err := json.Unmarshal([]byte(out), idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (t *androidTarget) push(ctx context.Context, m *store.Manifest) error {
	dir := t.root + "/builds/" + m.ID
	if err := t.mgr.PushDir(ctx, t.dev, m.ArtifactPath(), dir+"/"+m.Artifact.File); err != nil {
		return err
	}
	// The manifest goes last: a build dir without it is an unfinished copy.
	return t.mgr.PushDir(ctx, t.dev, filepath.Join(m.Dir, "manifest.json"), dir+"/manifest.json")
}

func (t *androidTarget) remove(ctx context.Context, id string) error {
	_, err := t.mgr.Shell(ctx, t.dev, "rm -rf "+proc.ShellQuote(t.root+"/builds/"+id))
	return err
}

func (t *androidTarget) writeIndex(ctx context.Context, data []byte) error {
	tmp, err := os.CreateTemp("", "ujibox-index-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	if err := t.mgr.PushDir(ctx, t.dev, tmp.Name(), t.root+"/index.json.tmp"); err != nil {
		return err
	}
	_, err = t.mgr.Shell(ctx, t.dev, "mv "+proc.ShellQuote(t.root+"/index.json.tmp")+" "+proc.ShellQuote(t.root+"/index.json"))
	return err
}

func (t *androidTarget) installFor(m *store.Manifest) Install {
	return Install{Kind: "apk", File: path.Join("builds", m.ID, m.Artifact.File)}
}

type simTarget struct {
	dev  device.Device
	root string
}

func (t *simTarget) readIndex(ctx context.Context) (*Index, error) {
	data, err := os.ReadFile(filepath.Join(t.root, "index.json"))
	if err != nil {
		return nil, err
	}
	idx := &Index{}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (t *simTarget) push(ctx context.Context, m *store.Manifest) error {
	dir := filepath.Join(t.root, "builds", m.ID)
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	for _, f := range []string{m.Artifact.File, "manifest.json"} {
		if out, err := exec.CommandContext(ctx, "cp", "-cR", filepath.Join(m.Dir, f), filepath.Join(tmp, f)).CombinedOutput(); err != nil {
			if out2, err2 := exec.CommandContext(ctx, "cp", "-R", filepath.Join(m.Dir, f), filepath.Join(tmp, f)).CombinedOutput(); err2 != nil {
				os.RemoveAll(tmp)
				return fmt.Errorf("%w: %s %s", err2, out, out2)
			}
		}
	}
	os.RemoveAll(dir)
	return os.Rename(tmp, dir)
}

func (t *simTarget) remove(ctx context.Context, id string) error {
	return os.RemoveAll(filepath.Join(t.root, "builds", id))
}

func (t *simTarget) writeIndex(ctx context.Context, data []byte) error {
	if err := os.MkdirAll(t.root, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(t.root, "index.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(t.root, "index.json"))
}

func (t *simTarget) installFor(m *store.Manifest) Install {
	app := filepath.Join(t.root, "builds", m.ID, m.Artifact.File)
	return Install{Kind: "command", Command: "xcrun simctl install " + t.dev.ID + " " + proc.ShellQuote(app)}
}

// phoneTarget writes into the ujibox app on a physical iOS device with
// devicectl. devicectl cannot delete or rename a file, so a removed build
// becomes an empty folder, and index.json is written in place.
type phoneTarget struct {
	mgr    *device.Manager
	dev    device.Device
	bundle string
}

const phoneRoot = "Documents/ujibox"

func (t *phoneTarget) readIndex(ctx context.Context) (*Index, error) {
	dir, err := os.MkdirTemp("", "uji-index-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	local := filepath.Join(dir, "index.json")
	if err := t.mgr.CopyFromApp(ctx, t.dev, t.bundle, phoneRoot+"/index.json", local); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(local)
	if err != nil {
		return nil, err
	}
	idx := &Index{}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (t *phoneTarget) push(ctx context.Context, m *store.Manifest) error {
	return t.mgr.CopyToApp(ctx, t.dev, t.bundle, filepath.Join(m.Dir, "manifest.json"),
		phoneRoot+"/builds/"+m.ID+"/manifest.json", false)
}

func (t *phoneTarget) remove(ctx context.Context, id string) error {
	empty, err := os.MkdirTemp("", "uji-empty-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(empty)
	return t.mgr.CopyToApp(ctx, t.dev, t.bundle, empty, phoneRoot+"/builds/"+id, true)
}

func (t *phoneTarget) writeIndex(ctx context.Context, data []byte) error {
	dir, err := os.MkdirTemp("", "uji-index-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	local := filepath.Join(dir, "index.json")
	if err := os.WriteFile(local, data, 0o644); err != nil {
		return err
	}
	return t.mgr.CopyToApp(ctx, t.dev, t.bundle, local, phoneRoot+"/index.json", false)
}

func (t *phoneTarget) installFor(m *store.Manifest) Install {
	return Install{Kind: "command", Command: "uji install -b " + m.ID + " " + proc.ShellQuote(t.dev.ID)}
}
