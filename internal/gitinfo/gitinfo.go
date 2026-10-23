// Package gitinfo reads the identity of the checkout a build comes from.
package gitinfo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Info describes a checkout. Branch is empty on a detached HEAD.
type Info struct {
	Root      string `json:"-"`
	MainRoot  string `json:"-"`
	Branch    string `json:"branch,omitempty"`
	Detached  bool   `json:"detached"`
	Worktree  string `json:"worktree"`
	SHA       string `json:"sha"`
	Dirty     bool   `json:"dirty"`
	DirtyHash string `json:"dirty_hash,omitempty"`
	Remote    string `json:"remote,omitempty"`
}

// Group is the name builds are grouped by: the branch, or "@<worktree>" when
// HEAD is detached.
func (i Info) Group() string {
	if i.Branch != "" {
		return i.Branch
	}
	return "@" + i.Worktree
}

// ShortSHA returns the first 8 characters of the commit.
func (i Info) ShortSHA() string {
	if len(i.SHA) > 8 {
		return i.SHA[:8]
	}
	return i.SHA
}

// Read returns the checkout that contains dir. ok is false outside a git repo.
func Read(ctx context.Context, dir string) (Info, bool) {
	root, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil || root == "" {
		return Info{}, false
	}
	info := Info{Root: root, MainRoot: root, Worktree: filepath.Base(root)}

	if common, err := git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		// The common dir of a linked worktree is <main>/.git; a bare repo has no main checkout.
		if filepath.Base(common) == ".git" {
			info.MainRoot = filepath.Dir(common)
		}
	}
	info.Branch, _ = git(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	info.Detached = info.Branch == ""
	info.SHA, _ = git(ctx, dir, "rev-parse", "HEAD")

	if status, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=no"); err == nil && status != "" {
		info.Dirty = true
		if diff, err := git(ctx, dir, "diff", "HEAD"); err == nil {
			sum := sha256.Sum256([]byte(diff))
			info.DirtyHash = hex.EncodeToString(sum[:8])
		}
	}
	if url, err := git(ctx, dir, "remote", "get-url", "origin"); err == nil {
		info.Remote = NormalizeRemote(url)
	}
	return info, true
}

var scpLike = regexp.MustCompile(`^[^@/]+@([^:]+):(.+)$`)

// NormalizeRemote turns the forms of one remote into one key:
// git@github.com:org/repo.git and https://github.com/org/repo both become
// github.com/org/repo.
func NormalizeRemote(url string) string {
	u := strings.TrimSpace(url)
	if m := scpLike.FindStringSubmatch(u); m != nil {
		u = m[1] + "/" + m[2]
	} else {
		if i := strings.Index(u, "://"); i >= 0 {
			u = u[i+3:]
		}
		if i := strings.Index(u, "@"); i >= 0 && i < strings.Index(u+"/", "/") {
			u = u[i+1:]
		}
	}
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return strings.ToLower(u)
}

// CommitURL returns the web page of a commit for a remote from
// NormalizeRemote, or "" when the host has no known commit page.
func CommitURL(remote, sha string) string {
	if remote == "" || sha == "" {
		return ""
	}
	host, path, ok := strings.Cut(remote, "/")
	if !ok || path == "" {
		return ""
	}
	// An ssh port is not the port of the web page.
	host, _, _ = strings.Cut(host, ":")
	switch {
	case strings.Contains(host, "dev.azure.com"), strings.Contains(host, "visualstudio.com"):
		return ""
	case strings.Contains(host, "gitlab"):
		return "https://" + host + "/" + path + "/-/commit/" + sha
	case strings.Contains(host, "bitbucket"):
		return "https://" + host + "/" + path + "/commits/" + sha
	}
	// GitHub, Gitea, Forgejo and Codeberg use /commit/.
	return "https://" + host + "/" + path + "/commit/" + sha
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
