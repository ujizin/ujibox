package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ujizin/ujibox/internal/gitinfo"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectAndroidFlavor(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "gradlew"), "")
	write(t, filepath.Join(root, "apps/shop/android/composeApp/build.gradle.kts"), `
plugins { id(libs.plugins.androidApplication.get().pluginId) }
android {
  defaultConfig { applicationId = "com.example.shop" }
  productFlavors {
    create("dev") { applicationIdSuffix = ".dev" }
    create("prod") { }
  }
}`)
	write(t, filepath.Join(root, "lib/build.gradle.kts"), `plugins { id("com.android.library") }`)

	d := detectAll(gitinfo.Info{Root: root, MainRoot: root}, false)
	if d == nil || len(d.Project.Apps) != 1 {
		t.Fatalf("want one app, got %+v", d)
	}
	a := d.Project.Apps["shop"]
	if a == nil || a.Android == nil {
		t.Fatalf("no shop android app: %v", d.Project.AppKeys())
	}
	if a.Android.ID != "com.example.shop.dev" {
		t.Errorf("id = %q", a.Android.ID)
	}
	if a.Android.Build[0] != "./gradlew :apps:shop:android:composeApp:assembleDevDebug" {
		t.Errorf("build = %q", a.Android.Build[0])
	}
	if !strings.HasSuffix(a.Android.Artifact, "apk/dev/debug/*.apk") {
		t.Errorf("artifact = %q", a.Android.Artifact)
	}
	// The rendered file loads back.
	f := filepath.Join(t.TempDir(), "p.toml")
	write(t, f, d.Render())
	if _, err := LoadProjectFile(f); err != nil {
		t.Fatalf("rendered file does not load: %v\n%s", err, d.Render())
	}
}

// A Gradle build in a subdirectory, with a root build file that only
// declares the plugins, like a KMP project.
func TestDetectNestedGradleRoot(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "mobile/settings.gradle.kts"), `include(":androidApp")`)
	write(t, filepath.Join(root, "mobile/gradlew"), "")
	write(t, filepath.Join(root, "mobile/local.properties"), "sdk.dir=/sdk")
	write(t, filepath.Join(root, "mobile/build.gradle.kts"), `plugins {
    alias(libs.plugins.androidApplication) apply false
}`)
	write(t, filepath.Join(root, "mobile/androidApp/build.gradle.kts"), `plugins { alias(libs.plugins.androidApplication) }`)

	d := detectAll(gitinfo.Info{Root: root, MainRoot: root}, false)
	if d == nil || len(d.Project.Apps) != 1 {
		t.Fatalf("want one app, got %+v", d)
	}
	a := d.Project.Apps[d.Project.Name]
	if a == nil || a.Android == nil {
		t.Fatalf("want the app under the project name %q, got %v", d.Project.Name, d.Project.AppKeys())
	}
	if want := "cd mobile && ./gradlew :androidApp:assembleDebug"; a.Android.Build[0] != want {
		t.Errorf("build = %q, want %q", a.Android.Build[0], want)
	}
	if len(a.Android.Setup) != 1 || a.Android.Setup[0].Creates != "mobile/local.properties" {
		t.Errorf("setup = %+v", a.Android.Setup)
	}
}

func TestProjectMatchAndVars(t *testing.T) {
	p := &Project{Match: Match{Remote: "git@github.com:org/repo.git", Paths: []string{"/work/other*"}}}
	if !p.Matches(gitinfo.Info{Remote: "github.com/org/repo", Root: "/x"}) {
		t.Error("remote should match")
	}
	if !p.Matches(gitinfo.Info{Root: "/work/other-wt", MainRoot: "/work/other"}) {
		t.Error("path glob should match")
	}
	v := Vars{Root: "/r", MainRoot: "/m"}
	if got := v.Expand(`cp {main_root}/a {root}/a ${HOME} {unknown}`); got != `cp /m/a /r/a ${HOME} {unknown}` {
		t.Errorf("Expand = %q", got)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.toml")
	write(t, f, "[apps.a.android]\nbuild=['x']\nartifact='y'\nartefact='typo'\n")
	if _, err := LoadProjectFile(f); err == nil || !strings.Contains(err.Error(), "artefact") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetAutoSync(t *testing.T) {
	t.Setenv("UJIBOX_CONFIG", t.TempDir())
	g, _ := LoadGlobal()
	if !g.AutoSyncOn() {
		t.Fatal("auto-sync must be on by default")
	}
	write(t, GlobalPath(), "# my config\nkeep_last = 5\n\n[aliases]\nfold = \"Pixel\"\n")
	if err := SetAutoSync(false); err != nil {
		t.Fatal(err)
	}
	if err := SetAutoSync(false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(GlobalPath())
	want := "# my config\nkeep_last = 5\nauto_sync = false\n\n[aliases]\nfold = \"Pixel\"\n"
	if string(data) != want {
		t.Fatalf("file:\n%s\nwant:\n%s", data, want)
	}
	g, err := LoadGlobal()
	if err != nil || g.AutoSyncOn() || g.KeepLast != 5 || g.Aliases["fold"] != "Pixel" {
		t.Fatalf("loaded %+v, %v", g, err)
	}
	if err := SetAutoSync(true); err != nil {
		t.Fatal(err)
	}
	if g, _ := LoadGlobal(); !g.AutoSyncOn() {
		t.Fatal("auto-sync must be on again")
	}
}
