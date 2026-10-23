package builder

import (
	"archive/zip"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/store"
)

// FindArtifact returns the newest path that matches pattern. The pattern is
// relative to root unless it is absolute. "**" matches any number of
// directories; the other parts follow filepath.Match.
func FindArtifact(root, pattern string) (string, error) {
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(root, pattern)
	}
	var matches []string
	if i := strings.Index(pattern, "**"); i >= 0 {
		base := filepath.Clean(pattern[:i])
		rest := strings.TrimPrefix(pattern[i+2:], string(filepath.Separator))
		_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == base {
				return nil
			}
			rel, _ := filepath.Rel(base, p)
			segs := strings.Split(rel, string(filepath.Separator))
			for k := range segs {
				if ok, _ := filepath.Match(rest, filepath.Join(segs[k:]...)); ok {
					matches = append(matches, p)
					break
				}
			}
			// A bundle is one artifact: do not look inside it.
			if d.IsDir() && strings.HasSuffix(p, ".app") {
				return filepath.SkipDir
			}
			return nil
		})
	} else {
		matches, _ = filepath.Glob(pattern)
	}
	var best string
	var bestTime time.Time
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		if best == "" || st.ModTime().After(bestTime) {
			best, bestTime = m, st.ModTime()
		}
	}
	if best == "" {
		return "", fmt.Errorf("the build finished, but nothing matches the artifact pattern %s", pattern)
	}
	return best, nil
}

// Inspect checks that an artifact is complete and reads its identity. An
// interrupted iOS build leaves a bundle without Info.plist; simctl then fails
// with "Missing bundle ID", so that bundle is rejected here.
func Inspect(ctx context.Context, platform, path, aapt2 string) (id string, v store.Version, err error) {
	switch platform {
	case config.Android:
		return inspectAPK(ctx, path, aapt2)
	case config.IOS:
		return inspectApp(ctx, path)
	}
	return "", v, fmt.Errorf("unknown platform %q", platform)
}

var (
	badgingPkgRe  = regexp.MustCompile(`package: name='([^']*)'`)
	badgingCodeRe = regexp.MustCompile(`versionCode='([^']*)'`)
	badgingNameRe = regexp.MustCompile(`versionName='([^']*)'`)
)

func inspectAPK(ctx context.Context, path, aapt2 string) (string, store.Version, error) {
	v := store.Version{}
	if !strings.HasSuffix(path, ".apk") {
		return "", v, fmt.Errorf("%s is not an .apk", path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", v, fmt.Errorf("%s is not a valid APK: %w", path, err)
	}
	hasManifest := false
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			hasManifest = true
			break
		}
	}
	zr.Close()
	if !hasManifest {
		return "", v, fmt.Errorf("%s has no AndroidManifest.xml", path)
	}
	if aapt2 == "" {
		// Without aapt2 the package id comes from the config.
		return "", v, nil
	}
	out, err := exec.CommandContext(ctx, aapt2, "dump", "badging", path).Output()
	if err != nil && len(out) == 0 {
		return "", v, nil
	}
	text := string(out)
	id := ""
	if m := badgingPkgRe.FindStringSubmatch(text); m != nil {
		id = m[1]
	}
	if m := badgingCodeRe.FindStringSubmatch(text); m != nil {
		v.Code = m[1]
	}
	if m := badgingNameRe.FindStringSubmatch(text); m != nil {
		v.Name = m[1]
	}
	return id, v, nil
}

func inspectApp(ctx context.Context, path string) (string, store.Version, error) {
	v := store.Version{}
	if !strings.HasSuffix(path, ".app") {
		return "", v, fmt.Errorf("%s is not an .app bundle", path)
	}
	plist := filepath.Join(path, "Info.plist")
	if _, err := os.Stat(plist); err != nil {
		return "", v, fmt.Errorf("%s has no Info.plist; the build was interrupted, run it again", path)
	}
	read := func(key string) string {
		out, err := exec.CommandContext(ctx, "plutil", "-extract", key, "raw", "-o", "-", plist).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	id := read("CFBundleIdentifier")
	if id == "" {
		return "", v, fmt.Errorf("%s has no CFBundleIdentifier", plist)
	}
	if exe := read("CFBundleExecutable"); exe != "" {
		if _, err := os.Stat(filepath.Join(path, exe)); err != nil {
			return "", v, fmt.Errorf("%s has no executable %s; the build was interrupted, run it again", path, exe)
		}
	}
	if p := read("DTPlatformName"); p != "" && p != "iphonesimulator" {
		return "", v, fmt.Errorf("%s is built for %s; ujibox installs simulator builds (iphonesimulator)", path, p)
	}
	v.Name = read("CFBundleShortVersionString")
	v.Code = read("CFBundleVersion")
	return id, v, nil
}
