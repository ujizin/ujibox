// Package proc runs external tools: adb, simctl, gradle, xcodebuild, scrcpy.
package proc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Output runs a command and returns its trimmed stdout. On failure the error
// carries the stderr text, because tools like adb explain themselves there.
func Output(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg != "" {
			return stdout.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, lastLines(msg, 5))
		}
		return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Combined runs a command and returns stdout and stderr together.
func Combined(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Env returns the current environment with the given KEY=VALUE pairs set.
// An empty value removes the key: adb reads an empty ANDROID_SERIAL as a device.
func Env(base []string, pairs ...string) []string {
	if base == nil {
		base = os.Environ()
	}
	out := make([]string, 0, len(base)+len(pairs))
	set := map[string]string{}
	order := []string{}
	for _, p := range pairs {
		k, v, _ := strings.Cut(p, "=")
		if _, ok := set[k]; !ok {
			order = append(order, k)
		}
		set[k] = v
	}
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := set[k]; ok {
			continue
		}
		out = append(out, kv)
	}
	for _, k := range order {
		if set[k] != "" {
			out = append(out, k+"="+set[k])
		}
	}
	return out
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ShellQuote quotes s for a POSIX shell. adb shell joins its arguments into one
// remote command line, so every value that reaches it goes through this.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
