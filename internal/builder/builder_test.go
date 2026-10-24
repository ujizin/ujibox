package builder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func touch(t *testing.T, p string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestFindArtifactNewest(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	touch(t, filepath.Join(root, "m/build/outputs/apk/dev/debug/old.apk"), now.Add(-time.Hour))
	touch(t, filepath.Join(root, "m/build/outputs/apk/dev/debug/new.apk"), now)
	touch(t, filepath.Join(root, "m/build/outputs/apk/prod/debug/other.apk"), now.Add(-2*time.Hour))

	got, err := FindArtifact(root, "m/build/outputs/apk/dev/debug/*.apk")
	if err != nil || filepath.Base(got) != "new.apk" {
		t.Fatalf("glob: got %q, %v", got, err)
	}
	got, err = FindArtifact(root, "m/build/outputs/apk/**/*.apk")
	if err != nil || filepath.Base(got) != "new.apk" {
		t.Fatalf("doublestar: got %q, %v", got, err)
	}
	if _, err := FindArtifact(root, "nothing/*.apk"); err == nil {
		t.Fatal("expected an error for no match")
	}
}

func TestSummarizeGradle(t *testing.T) {
	log := "line\n* What went wrong:\nExecution failed for task ':app:compile'.\n> boom\n\n* Try:\n> Run with --stacktrace\n"
	got := Summarize(log)
	if !strings.Contains(got, "boom") || strings.Contains(got, "stacktrace") {
		t.Fatalf("Summarize = %q", got)
	}
}

func TestSummarizeXcode(t *testing.T) {
	log := "CompileSwift\n/x/a.swift:1:1: error: cannot find 'x'\n/x/a.swift:1:1: error: cannot find 'x'\n** BUILD FAILED **\n"
	got := Summarize(log)
	if strings.Count(got, "cannot find") != 1 || !strings.Contains(got, "BUILD FAILED") {
		t.Fatalf("Summarize = %q", got)
	}
}

func TestSummarizeScriptPhase(t *testing.T) {
	log := "PhaseScriptExecution Validate x.sh\n    cd /p\n    /bin/sh -c x.sh\nproject is out of sync\n  + missing a\nCommand PhaseScriptExecution failed with a nonzero exit code\n** BUILD FAILED **\n"
	got := Summarize(log)
	if !strings.Contains(got, "out of sync") || !strings.Contains(got, "missing a") || strings.Contains(got, "/bin/sh") {
		t.Fatalf("Summarize = %q", got)
	}
}

func TestInspectAppTarget(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Shop.app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.example.shop</string>
<key>CFBundleShortVersionString</key><string>1.2</string>
<key>CFBundleVersion</key><string>7</string>
<key>DTPlatformName</key><string>iphoneos</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := Inspect(ctx, "ios", "simulator", app, ""); err == nil || !strings.Contains(err.Error(), "iphoneos") {
		t.Errorf("a device build must not pass as a simulator build: %v", err)
	}
	id, v, err := Inspect(ctx, "ios", "device", app, "")
	if err != nil || id != "com.example.shop" || v.Name != "1.2" || v.Code != "7" {
		t.Errorf("device inspect = %q %+v %v", id, v, err)
	}
}
