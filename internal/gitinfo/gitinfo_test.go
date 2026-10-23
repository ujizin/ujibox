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

func TestCommitURL(t *testing.T) {
	cases := map[string]string{
		"github.com/org/repo":         "https://github.com/org/repo/commit/abc",
		"gitlab.example.com/g/sub/r":  "https://gitlab.example.com/g/sub/r/-/commit/abc",
		"bitbucket.org/team/repo":     "https://bitbucket.org/team/repo/commits/abc",
		"git.example.com:2222/org/r":  "https://git.example.com/org/r/commit/abc",
		"dev.azure.com/org/proj/_git": "",
		"":                            "",
		"github.com":                  "",
	}
	for remote, want := range cases {
		if got := CommitURL(remote, "abc"); got != want {
			t.Errorf("CommitURL(%q) = %q, want %q", remote, got, want)
		}
	}
}
