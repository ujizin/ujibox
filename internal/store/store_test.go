package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestULIDSortsByTime(t *testing.T) {
	a := newULID(time.UnixMilli(1_000))
	b := newULID(time.UnixMilli(2_000))
	if len(a) != 26 || len(b) != 26 {
		t.Fatalf("length: %q %q", a, b)
	}
	if !(a < b) {
		t.Fatalf("%q should sort before %q", a, b)
	}
	if strings.ContainsAny(a, "ILOU") {
		t.Fatalf("%q has letters outside Crockford base32", a)
	}
}

func addBuild(t *testing.T, s *Store, app, platform, branch, sha string, age time.Duration) *Manifest {
	t.Helper()
	src := filepath.Join(t.TempDir(), "app.apk")
	if err := os.WriteFile(src, []byte("apk"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := s.Add(&Manifest{App: AppRef{ID: app, Name: app}, Platform: platform, Git: Git{Branch: branch, SHA: sha}}, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		m.CreatedAt = m.CreatedAt.Add(-age)
		if err := writeJSON(filepath.Join(m.Dir, "manifest.json"), m); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(2 * time.Millisecond) // distinct ULID time parts
	return m
}

func TestFind(t *testing.T) {
	s := Open(t.TempDir())
	old := addBuild(t, s, "app", "android", "feature/x", "aaaa1111", 0)
	newer := addBuild(t, s, "app", "android", "feature/x", "bbbb2222", 0)
	other := addBuild(t, s, "app", "android", "main", "cccc3333", 0)

	scope := Filter{AppID: "app", Platform: "android", Group: "feature/x"}
	cases := map[string]*Manifest{
		"last":        newer,
		"":            newer,
		"2":           old,
		"aaaa":        old,
		"main":        other,
		old.ShortID(): old,
		newer.ID:      newer,
	}
	for ref, want := range cases {
		got, err := s.Find(ref, scope)
		if err != nil {
			t.Fatalf("Find(%q): %v", ref, err)
		}
		if got.ID != want.ID {
			t.Errorf("Find(%q) = %s, want %s", ref, got.Git.SHA, want.Git.SHA)
		}
	}
	if _, err := s.Find("last", Filter{AppID: "app", Platform: "ios"}); err == nil {
		t.Error("Find on an empty scope should fail")
	}
}

func TestPrune(t *testing.T) {
	s := Open(t.TempDir())
	var olds []*Manifest
	for i := 0; i < 5; i++ {
		olds = append(olds, addBuild(t, s, "app", "android", "main", "sha", 40*24*time.Hour))
	}
	pinned := olds[0]
	if err := s.SetPinned(pinned, true); err != nil {
		t.Fatal(err)
	}
	fresh := addBuild(t, s, "app", "android", "dev", "sha", 0)

	removed, errs := s.Prune(Policy{MaxAge: 30 * 24 * time.Hour, KeepLast: 3}, time.Now(), false)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	// 5 old builds on main: the newest 3 stay, the oldest is pinned, one goes.
	if len(removed) != 1 || removed[0].ID != olds[1].ID {
		t.Fatalf("removed %d builds, want only %s", len(removed), olds[1].ID)
	}
	left := map[string]bool{}
	for _, m := range s.List(Filter{}) {
		left[m.ID] = true
	}
	for _, m := range []*Manifest{pinned, olds[2], olds[3], olds[4], fresh} {
		if !left[m.ID] {
			t.Errorf("build %s should stay", m.ID)
		}
	}
}

func TestListSkipsHalfBuilds(t *testing.T) {
	s := Open(t.TempDir())
	m := addBuild(t, s, "app", "android", "main", "sha", 0)
	if err := os.Remove(m.ArtifactPath()); err != nil {
		t.Fatal(err)
	}
	if n := len(s.List(Filter{})); n != 0 {
		t.Fatalf("listed %d builds without an artifact", n)
	}
}
