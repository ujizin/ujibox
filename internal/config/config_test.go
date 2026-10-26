package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
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

func TestDeviceBuild(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.toml")
	write(t, f, `name = "shop"
[apps.shop.ios]
id = "com.example.shop"
build = ["xcodebuild simulator"]
artifact = "sim/*.app"
env = { A = "1", B = "1" }

[apps.shop.ios.device]
build = ["xcodebuild device"]
artifact = "device/*.app"
env = { B = "2" }
`)
	p, err := LoadProjectFile(f)
	if err != nil {
		t.Fatal(err)
	}
	ios := p.Apps["shop"].IOS
	dev, err := ios.ForDevice()
	if err != nil {
		t.Fatal(err)
	}
	if dev.Build[0] != "xcodebuild device" || dev.Artifact != "device/*.app" || dev.ID != "com.example.shop" {
		t.Errorf("device platform = %+v", dev)
	}
	if dev.Env["A"] != "1" || dev.Env["B"] != "2" {
		t.Errorf("env = %v", dev.Env)
	}
	if ios.Build[0] != "xcodebuild simulator" || ios.Env["B"] != "1" {
		t.Errorf("ForDevice changed the simulator platform: %+v", ios)
	}
	ios.Device = nil
	if _, err := ios.ForDevice(); err == nil {
		t.Error("want an error without a device build")
	}
}

func TestRepoFileAndOverride(t *testing.T) {
	t.Setenv("UJIBOX_CONFIG", t.TempDir())
	root := t.TempDir()
	g := gitinfo.Info{Root: root, MainRoot: root, Remote: "github.com/example/shop"}
	write(t, filepath.Join(root, RepoFileName), `name = "shop"
default_app = "shop"

[apps.shop]
display = "Shop"
env = { A = "repo", B = "repo" }

[apps.shop.android]
build = ["./gradlew :app:assembleDebug"]
artifact = "app/build/outputs/apk/debug/*.apk"

[apps.shop.ios]
build = ["xcodebuild simulator"]
artifact = "sim/*.app"

[apps.shop.ios.device]
build = ["xcodebuild device"]
artifact = "device/*.app"
`)

	// Only the repo file.
	p, errs := Find(g)
	if len(errs) > 0 || p == nil || p.Detected {
		t.Fatalf("repo file: %+v %v", p, errs)
	}
	if p.RepoFile == "" || p.File != "" || p.Name != "shop" || p.EditFile() != filepath.Join(root, RepoFileName) {
		t.Errorf("repo project = %+v", p)
	}

	// A machine file overrides some keys and adds an app.
	write(t, filepath.Join(paths.Projects(), "shop.toml"), `[match]
remote = "github.com/example/shop"

[apps.shop]
env = { B = "machine" }

[apps.shop.ios.device]
build = ["xcodebuild device DEVELOPMENT_TEAM=ABC"]

[apps.admin.android]
build = ["./gradlew :admin:assembleDebug"]
artifact = "admin/*.apk"
`)
	p, errs = Find(g)
	if len(errs) > 0 || p == nil {
		t.Fatalf("merged: %+v %v", p, errs)
	}
	shop := p.Apps["shop"]
	if shop.Env["A"] != "repo" || shop.Env["B"] != "machine" || shop.Display != "Shop" {
		t.Errorf("app env/display = %v %q", shop.Env, shop.Display)
	}
	if shop.Android.Build[0] != "./gradlew :app:assembleDebug" {
		t.Errorf("android build = %v", shop.Android.Build)
	}
	if d := shop.IOS.Device; d.Build[0] != "xcodebuild device DEVELOPMENT_TEAM=ABC" || d.Artifact != "device/*.app" {
		t.Errorf("device = %+v", d)
	}
	if p.Apps["admin"] == nil || p.DefaultApp != "shop" || p.Name != "shop" {
		t.Errorf("merged project = %+v", p)
	}
	if p.EditFile() != filepath.Join(paths.Projects(), "shop.toml") || p.RepoFile == "" {
		t.Errorf("files: %q %q", p.File, p.RepoFile)
	}

	// The override must not change the repo values that it leaves out.
	repo, _ := LoadRepoFile(g)
	if repo.Apps["shop"].Env["B"] != "repo" || repo.Apps["shop"].IOS.Device.Build[0] != "xcodebuild device" {
		t.Errorf("repo file changed: %+v", repo.Apps["shop"])
	}

	// A broken machine file falls back to the repo file, with an error.
	write(t, filepath.Join(paths.Projects(), "shop.toml"), `[match]
remote = "github.com/example/shop"

[apps.extra.android]
build = ["x"]
`)
	p, errs = Find(g)
	if len(errs) != 1 || p == nil || p.File != "" {
		t.Errorf("broken override: %+v %v", p, errs)
	}
}

func TestRenderRepoAndOverride(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "app/build.gradle.kts"), `plugins { id("com.android.application") }
android { defaultConfig { applicationId = "com.example.shop" } }`)
	d := detectAll(gitinfo.Info{Root: root, MainRoot: root, Remote: "github.com/example/shop"}, false)
	text := d.RenderRepo()
	if strings.Contains(text, "[match]") || strings.Contains(text, root) {
		t.Errorf("a repo file must not hold a match table or a local path:\n%s", text)
	}
	f := filepath.Join(t.TempDir(), RepoFileName)
	write(t, f, text)
	if _, err := LoadProjectFile(f); err != nil {
		t.Fatalf("rendered repo file does not load: %v\n%s", err, text)
	}
	over := RenderOverride(d.Project, Match{Remote: "github.com/example/shop"})
	if !strings.Contains(over, `remote = "github.com/example/shop"`) || strings.Contains(over, "\nbuild =") {
		t.Errorf("override:\n%s", over)
	}
}

func TestDetectDesktopAndWeb(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "settings.gradle.kts"), `include(":sample:android", ":sample:desktop", ":sample:web")`)
	write(t, filepath.Join(root, "gradlew"), "")
	write(t, filepath.Join(root, "build.gradle.kts"), `plugins {
    alias(libs.plugins.androidApplication) apply false
}`)
	write(t, filepath.Join(root, "sample/android/build.gradle.kts"), `plugins { id("com.android.application") }
android { defaultConfig { applicationId = "com.example.shop" } }`)
	write(t, filepath.Join(root, "sample/desktop/build.gradle.kts"), `
compose.desktop {
    application {
        mainClass = "MainKt"
        nativeDistributions { macOS { bundleID = "com.example.shop.desktop" } }
    }
}`)
	write(t, filepath.Join(root, "sample/web/build.gradle.kts"), `kotlin {
    js { browser(); binaries.executable() }
    wasmJs { browser(); binaries.executable() }
}`)
	write(t, filepath.Join(root, "local.properties"), "sdk.dir=/x")
	d := DetectVerbose(gitinfo.Info{Root: root, MainRoot: root})
	if d == nil || len(d.Project.Apps) != 1 {
		t.Fatalf("want one app with every platform, got %v", d.Project.AppKeys())
	}
	var a *App
	for _, x := range d.Project.Apps {
		a = x
	}
	if a.Desktop == nil || a.Web == nil || a.Android == nil {
		t.Fatalf("platforms = %v", a.Platforms())
	}
	if a.Desktop.Build[0] != "./gradlew :sample:desktop:createDistributable" || a.Desktop.ID != "com.example.shop.desktop" {
		t.Errorf("desktop = %+v", a.Desktop)
	}
	if !strings.HasSuffix(a.Desktop.Artifact, "sample/desktop/build/compose/binaries/main/app/*.app") {
		t.Errorf("desktop artifact = %q", a.Desktop.Artifact)
	}
	if a.Web.Build[0] != "./gradlew :sample:web:wasmJsBrowserDistribution" || a.Web.Artifact != "sample/web/build/dist/wasmJs/productionExecutable" {
		t.Errorf("web = %+v", a.Web)
	}
	if len(a.Desktop.Setup) != 1 || len(a.Web.Setup) != 1 {
		t.Errorf("desktop and web builds must copy local.properties into a worktree: %+v %+v", a.Desktop.Setup, a.Web.Setup)
	}
	file := filepath.Join(t.TempDir(), "p.toml")
	write(t, file, d.Render())
	if _, err := LoadProjectFile(file); err != nil {
		t.Errorf("the rendered file does not load: %v", err)
	}
}

func TestDesktopOnlyProject(t *testing.T) {
	file := filepath.Join(t.TempDir(), "p.toml")
	write(t, file, "[apps.a.desktop]\nbuild=['x']\nartifact='y.app'\n[apps.a.web]\nbuild=['x']\nartifact='dist'\nid='a'\n")
	p, err := LoadProjectFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(p.Apps["a"].Platforms(), ","); got != "desktop,web" {
		t.Errorf("platforms = %s", got)
	}
	write(t, file, "[apps.a.desktop]\nbuild=['x']\nartifact='y.app'\n[apps.a.desktop.device]\nbuild=['x']\nartifact='z'\n")
	if _, err := LoadProjectFile(file); err == nil {
		t.Error("a device build outside ios must fail")
	}
}
