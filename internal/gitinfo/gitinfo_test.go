package gitinfo

import "testing"

func TestNormalizeRemote(t *testing.T) {
	want := "github.com/org/repo"
	for _, in := range []string{
		"git@github.com:org/repo.git",
		"https://github.com/org/repo",
		"https://user@github.com/org/repo.git",
		"ssh://git@github.com/org/repo.git",
		"https://GitHub.com/Org/Repo/",
	} {
		if got := NormalizeRemote(in); got != want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", in, got, want)
		}
	}
}
