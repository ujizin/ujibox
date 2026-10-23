// Package builder runs a project's setup, build and retry steps, and puts the
// artifact in the store.
package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/proc"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

// Version is the ujibox version written into each manifest.
var Version = "dev"

// Request is one build of one platform of one app.
type Request struct {
	Project  *config.Project
	App      *config.App
	Platform string
	Git      gitinfo.Info
	Store    *store.Store
	Tools    device.Tools
	Clean    bool
	Verbose  bool
	Out      io.Writer // status lines
}

// Error is a failed build. Summary holds the lines that explain the failure.
type Error struct {
	Platform string
	Summary  string
	Log      string
	Hint     string
	Err      error
}

func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s build failed", e.Platform)
	if e.Err != nil {
		fmt.Fprintf(&b, ": %v", e.Err)
	}
	if e.Summary != "" {
		b.WriteString("\n" + indent(e.Summary))
	}
	if e.Hint != "" {
		b.WriteString("\nhint: " + e.Hint)
	}
	if e.Log != "" {
		b.WriteString("\nlog: " + paths.Pretty(e.Log))
	}
	return b.String()
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

// Run builds, checks the artifact and stores it.
func Run(ctx context.Context, r Request) (*store.Manifest, error) {
	pl := r.App.Platform(r.Platform)
	if pl == nil {
		return nil, fmt.Errorf("app %q has no %s config", r.App.Key, r.Platform)
	}
	vars := config.NewVars(r.Git, r.App, r.Platform)

	// Two builds in one checkout write the same build/ outputs; builds in
	// different worktrees run in parallel.
	lockPath := filepath.Join(paths.Cache(), "locks", "checkout-"+hash(r.Git.Root)+".lock")
	lk, err := store.TryLockExclusive(lockPath)
	if errors.Is(err, store.ErrLocked) {
		ui.Info(r.Out, "another uji build runs in this checkout; waiting for it")
		lk, err = store.LockExclusive(lockPath)
	}
	if err != nil {
		return nil, fmt.Errorf("lock checkout: %w", err)
	}
	defer lk.Unlock()

	env := buildEnv(r, pl, vars)
	logDir := filepath.Join(paths.Cache(), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("%s-%s-%s.log", r.App.Key, r.Platform, time.Now().Format("20060102-150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()

	if r.Clean {
		for _, s := range append(append([]config.Step{}, r.App.Clean...), pl.Clean...) {
			ui.Info(r.Out, "clean: %s", vars.Expand(s.Run))
			if err := runShell(ctx, r.Git.Root, env, vars.Expand(s.Run), io.MultiWriter(logFile, verboseOut(r))); err != nil {
				return nil, &Error{Platform: r.Platform, Err: fmt.Errorf("clean step failed: %w", err), Log: logPath}
			}
		}
	}
	if err := runSetup(ctx, r, pl, vars, env, logFile); err != nil {
		return nil, &Error{Platform: r.Platform, Err: err, Log: logPath}
	}

	start := time.Now()
	label := fmt.Sprintf("building %s %s", r.App.Name(), r.Platform)
	commands := make([]string, len(pl.Build))
	for i, c := range pl.Build {
		commands[i] = vars.Expand(c)
	}
	retries := append(append([]config.Retry{}, r.App.Retry...), pl.Retry...)
	retried := false
	for {
		offset, _ := logFile.Seek(0, io.SeekCurrent)
		err = runSteps(ctx, r, label, commands, env, logFile)
		if err == nil || ctx.Err() != nil || retried {
			break
		}
		text := readFrom(logPath, offset)
		rule := matchRetry(retries, text)
		if rule == nil {
			break
		}
		retried = true
		ui.Warn(r.Out, "build failed on /%s/; running %q and building again", rule.Match, vars.Expand(rule.Then))
		if rerr := runShell(ctx, r.Git.Root, env, vars.Expand(rule.Then), io.MultiWriter(logFile, verboseOut(r))); rerr != nil {
			break
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		text := readFrom(logPath, 0)
		return nil, &Error{Platform: r.Platform, Err: err, Summary: Summarize(text), Hint: hintFor(text), Log: logPath}
	}

	artifact, err := FindArtifact(r.Git.Root, vars.Expand(pl.Artifact))
	if err != nil {
		return nil, &Error{Platform: r.Platform, Err: err, Log: logPath}
	}
	id, version, err := Inspect(ctx, r.Platform, artifact, r.Tools.AAPT2)
	if err != nil {
		return nil, &Error{Platform: r.Platform, Err: err, Log: logPath}
	}
	if id == "" {
		id = pl.ID
	}
	host, _ := os.Hostname()
	m := &store.Manifest{
		App:       store.AppRef{ID: r.Project.StoreID(r.App), Name: r.App.Name(), Project: r.Project.Name},
		Platform:  r.Platform,
		Target:    target(r.Platform),
		PackageID: id,
		Version:   version,
		Git: store.Git{
			Branch: r.Git.Branch, Detached: r.Git.Detached, Worktree: r.Git.Worktree, WorktreePath: r.Git.Root,
			SHA: r.Git.SHA, Dirty: r.Git.Dirty, DirtyHash: r.Git.DirtyHash, Remote: r.Git.Remote,
		},
		Build: store.BuildInfo{Commands: commands, DurationMS: time.Since(start).Milliseconds(), Host: host, Tool: "uji " + Version},
	}
	logFile.Sync()
	return r.Store.Add(m, artifact, logPath)
}

// Setup runs only the setup steps of a platform.
func Setup(ctx context.Context, r Request) error {
	pl := r.App.Platform(r.Platform)
	if pl == nil {
		return fmt.Errorf("app %q has no %s config", r.App.Key, r.Platform)
	}
	vars := config.NewVars(r.Git, r.App, r.Platform)
	return runSetup(ctx, r, pl, vars, buildEnv(r, pl, vars), io.Discard)
}

func runSetup(ctx context.Context, r Request, pl *config.Platform, vars config.Vars, env []string, log io.Writer) error {
	for _, s := range append(append([]config.Step{}, r.App.Setup...), pl.Setup...) {
		if s.Creates != "" {
			p := vars.Expand(s.Creates)
			if !filepath.IsAbs(p) {
				p = filepath.Join(r.Git.Root, p)
			}
			if config.Exists(p) {
				continue
			}
		}
		cmd := vars.Expand(s.Run)
		ui.Info(r.Out, "setup: %s", cmd)
		if err := runShell(ctx, r.Git.Root, env, cmd, io.MultiWriter(log, verboseOut(r))); err != nil {
			return fmt.Errorf("setup step %q failed: %w", cmd, err)
		}
	}
	return nil
}

func runSteps(ctx context.Context, r Request, label string, commands, env []string, log io.Writer) error {
	var status *ui.Status
	var w io.Writer = log
	if r.Verbose {
		w = io.MultiWriter(log, os.Stderr)
	} else {
		status = ui.NewStatus(label)
		w = io.MultiWriter(log, status)
		defer status.Done()
	}
	for _, c := range commands {
		fmt.Fprintf(log, "\n$ %s\n", c)
		if err := runShell(ctx, r.Git.Root, env, c, w); err != nil {
			return err
		}
	}
	return nil
}

func verboseOut(r Request) io.Writer {
	if r.Verbose {
		return os.Stderr
	}
	return io.Discard
}

// runShell runs a command line with /bin/sh in its own process group. On
// cancel the whole group gets SIGINT, so gradle and xcodebuild children stop
// too, then SIGKILL after a grace period.
func runShell(ctx context.Context, dir string, env []string, line string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", line)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT) }
	cmd.WaitDelay = 10 * time.Second
	err := cmd.Run()
	if err != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return err
}

func buildEnv(r Request, pl *config.Platform, vars config.Vars) []string {
	var pairs []string
	pairs = append(pairs, "ANDROID_SERIAL=")
	if r.Platform == config.IOS && r.Tools.DeveloperDir != "" {
		pairs = append(pairs, "DEVELOPER_DIR="+r.Tools.DeveloperDir)
	}
	if r.Tools.AndroidSDK != "" && os.Getenv("ANDROID_HOME") == "" {
		pairs = append(pairs, "ANDROID_HOME="+r.Tools.AndroidSDK)
	}
	pairs = append(pairs, vars.Env()...)
	for k, v := range r.App.Env {
		pairs = append(pairs, k+"="+vars.Expand(v))
	}
	for k, v := range pl.Env {
		pairs = append(pairs, k+"="+vars.Expand(v))
	}
	return proc.Env(nil, pairs...)
}

func matchRetry(rules []config.Retry, text string) *config.Retry {
	for i, rule := range rules {
		re, err := regexp.Compile(rule.Match)
		if err != nil {
			continue
		}
		if re.MatchString(text) {
			return &rules[i]
		}
	}
	return nil
}

func readFrom(path string, offset int64) string {
	data, err := os.ReadFile(path)
	if err != nil || offset > int64(len(data)) {
		return ""
	}
	return string(data[offset:])
}

func target(platform string) string {
	if platform == config.IOS {
		return device.KindSimulator
	}
	return device.KindEmulator
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}
