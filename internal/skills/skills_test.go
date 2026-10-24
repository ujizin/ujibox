package skills

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const repoSkills = "../../skills"

// The index must list every skill folder and every file in it, and each
// SKILL.md must carry the name and a description, as the agents need.
func TestRepoIndexMatchesFolders(t *testing.T) {
	ix, err := LoadIndex(context.Background(), Dir{Path: repoSkills})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(repoSkills)
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	var names []string
	for _, s := range ix.Skills {
		names = append(names, s.Name)
		var onDisk []string
		root := filepath.Join(repoSkills, s.Name)
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(root, p)
				onDisk = append(onDisk, filepath.ToSlash(rel))
			}
			return nil
		})
		listed := append([]string{}, s.Files...)
		sort.Strings(listed)
		sort.Strings(onDisk)
		if strings.Join(listed, ",") != strings.Join(onDisk, ",") {
			t.Errorf("skill %s: index lists %v, folder has %v", s.Name, listed, onDisk)
		}
		data, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		front := string(data)
		if !strings.HasPrefix(front, "---\n") {
			t.Fatalf("skill %s: SKILL.md has no frontmatter", s.Name)
		}
		front = front[4 : strings.Index(front[4:], "\n---")+4]
		if !regexp.MustCompile(`(?m)^name: ` + regexp.QuoteMeta(s.Name) + `$`).MatchString(front) {
			t.Errorf("skill %s: frontmatter name does not match the folder", s.Name)
		}
		if !regexp.MustCompile(`(?m)^description: \S`).MatchString(front) {
			t.Errorf("skill %s: frontmatter has no description", s.Name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(names)
	if strings.Join(dirs, ",") != strings.Join(names, ",") {
		t.Errorf("folders %v, index %v", dirs, names)
	}
}

func TestInstall(t *testing.T) {
	ctx := context.Background()
	src := Dir{Path: repoSkills}
	ix, err := LoadIndex(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := ix.Find("uji")
	if !ok {
		t.Fatal("no uji skill")
	}
	dir := filepath.Join(t.TempDir(), ".claude", "skills")
	dest, err := Install(ctx, src, s, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(ctx, src, s, dir, false); err == nil {
		t.Error("a second install without force must fail")
	}
	os.WriteFile(filepath.Join(dest, "old.md"), []byte("x"), 0o644)
	if _, err := Install(ctx, src, s, dir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "old.md")); err == nil {
		t.Error("force must replace the whole folder")
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".uji-*"))
	if len(left) > 0 {
		t.Errorf("temporary folders left: %v", left)
	}
}

func TestCheckRefusesBadPaths(t *testing.T) {
	for _, s := range []Skill{
		{Name: "x", Files: []string{"SKILL.md", "../evil"}},
		{Name: "x", Files: []string{"SKILL.md", "/etc/passwd"}},
		{Name: "x", Files: []string{"SKILL.md", "a/../../b"}},
		{Name: "../x", Files: []string{"SKILL.md"}},
		{Name: "x", Files: []string{"README.md"}},
	} {
		if err := s.check(); err == nil {
			t.Errorf("check(%+v) passed", s)
		}
	}
}
