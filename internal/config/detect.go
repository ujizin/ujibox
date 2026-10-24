package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ujizin/ujibox/internal/gitinfo"
)

// Detect guesses a project from the files of a checkout. It returns nil when
// it finds no Android application module and no Xcode project.
func Detect(g gitinfo.Info) *Project {
	d := detectAll(g, false)
	if d == nil {
		return nil
	}
	return d.Project
}

// Detection is a detected project plus the notes `uji config project` writes
// as comments next to each value.
type Detection struct {
	Project *Project
	Notes   map[string][]string // key: "<app>.<platform>" or "<app>"
}

// DetectVerbose detects a project and also asks xcodebuild for the bundle id
// and product name, which takes a few seconds.
func DetectVerbose(g gitinfo.Info) *Detection {
	return detectAll(g, true)
}

var skipDirs = map[string]bool{
	".git": true, "build": true, ".gradle": true, "node_modules": true, "Pods": true,
	"DerivedData": true, ".idea": true, ".build": true, "Carthage": true, ".kotlin": true,
}

func detectAll(g gitinfo.Info, slow bool) *Detection {
	root := g.Root
	var gradleFiles, xcodeProjects, workspaces []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		depth := strings.Count(rel, string(filepath.Separator))
		if d.IsDir() {
			name := d.Name()
			if p != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			switch {
			case strings.HasSuffix(name, ".xcworkspace"):
				workspaces = append(workspaces, rel)
				return filepath.SkipDir
			case strings.HasSuffix(name, ".xcodeproj"):
				xcodeProjects = append(xcodeProjects, rel)
				return filepath.SkipDir
			}
			if depth > 5 {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "build.gradle" || d.Name() == "build.gradle.kts" {
			gradleFiles = append(gradleFiles, rel)
		}
		return nil
	})

	det := &Detection{
		Project: &Project{Name: slug(filepath.Base(g.MainRoot)), Apps: map[string]*App{}, Detected: true},
		Notes:   map[string][]string{},
	}
	if g.Remote != "" {
		det.Project.Match.Remote = g.Remote
	} else {
		det.Project.Match.Paths = []string{g.MainRoot}
	}

	for _, f := range gradleFiles {
		if a, notes := detectAndroid(root, f); a != nil {
			key := appKey(filepath.Dir(f))
			if key == "" {
				key = det.Project.Name
			}
			key = uniqueKey(det.Project.Apps, key)
			det.Project.Apps[key] = &App{Key: key, Android: a}
			det.Notes[key+".android"] = notes
		}
	}

	// A workspace wraps a project (CocoaPods, or a project plus packages), so
	// it wins over the projects inside it.
	containers := workspaces
	if len(containers) == 0 {
		containers = xcodeProjects
	}
	for _, c := range containers {
		pl, notes := detectIOS(root, c, slow)
		if pl == nil {
			continue
		}
		key := appKey(filepath.Dir(c))
		if key == "" {
			key = appKey(strings.TrimSuffix(c, filepath.Ext(c)))
		}
		if key == "" {
			key = det.Project.Name
		}
		// Pair with the Android app of the same name, or with the only one.
		target := det.Project.Apps[key]
		if target == nil && len(det.Project.Apps) == 1 && len(containers) == 1 {
			for _, a := range det.Project.Apps {
				target = a
			}
		}
		if target == nil || target.IOS != nil {
			key = uniqueKey(det.Project.Apps, key)
			target = &App{Key: key}
			det.Project.Apps[key] = target
		}
		target.IOS = pl
		det.Notes[target.Key+".ios"] = notes
	}
	if len(det.Project.Apps) == 0 {
		return nil
	}

	// A worktree has no local.properties; Gradle then fails with "SDK location
	// not found". Copy it from the main checkout.
	for _, a := range det.Project.Apps {
		if a.Android == nil {
			continue
		}
		props := filepath.Join(a.Android.gradleRoot, "local.properties")
		if Exists(filepath.Join(g.MainRoot, props)) {
			copyCmd := `cp "{main_root}/` + props + `" "{root}/` + props + `"`
			a.Android.Setup = append(a.Android.Setup, Step{Run: copyCmd, Creates: props})
			a.Android.Retry = append(a.Android.Retry, Retry{Match: `SDK location not found|local\.properties`, Then: copyCmd})
			det.Notes[a.Key+".android"] = append(det.Notes[a.Key+".android"],
				"the main checkout has local.properties, so each new worktree copies it before the build")
		}
	}

	for k, a := range det.Project.Apps {
		a.Display = displayName(k)
	}
	if len(det.Project.Apps) > 1 {
		// The default is the app with the most platforms; a tie goes by name.
		best := ""
		for _, k := range det.Project.AppKeys() {
			if best == "" || len(det.Project.Apps[k].Platforms()) > len(det.Project.Apps[best].Platforms()) {
				best = k
			}
		}
		det.Project.DefaultApp = best
	}
	return det
}

var (
	applyFalseRe  = regexp.MustCompile(`(?m)^.*\bapply\s*\(?\s*false.*$`)
	appPluginRe   = regexp.MustCompile(`(?i)com\.android\.application|plugins\.android[._-]?application|\bapplicationId\s*=`)
	suffixRe      = regexp.MustCompile(`applicationIdSuffix\s*=?\s*["']([^"']+)["']`)
	appIDRe       = regexp.MustCompile(`applicationId\s*=?\s*["']([^"']+)["']`)
	flavorBlockRe = regexp.MustCompile(`productFlavors\s*\{`)
	flavorKtsRe   = regexp.MustCompile(`(?:create|register|maybeCreate)\(\s*"([A-Za-z0-9_]+)"`)
	flavorGrRe    = regexp.MustCompile(`(?m)^\s*([A-Za-z][A-Za-z0-9_]*)\s*\{`)
)

func detectAndroid(root, gradleFile string) (*Platform, []string) {
	data, err := os.ReadFile(filepath.Join(root, gradleFile))
	if err != nil {
		return nil, nil
	}
	// A root build file declares the plugin with "apply false"; that is not an app.
	text := applyFalseRe.ReplaceAllString(string(data), "")
	if !appPluginRe.MatchString(text) {
		return nil, nil
	}
	moduleDir := filepath.Dir(gradleFile)
	gradleRoot := findGradleRoot(root, moduleDir)
	module := ""
	if rel, _ := filepath.Rel(gradleRoot, moduleDir); rel != "." {
		module = ":" + strings.ReplaceAll(rel, string(filepath.Separator), ":")
	}
	var notes []string
	notes = append(notes, fmt.Sprintf("found an Android application module in %s", gradleFile))

	flavor, flavorSuffix := "", ""
	if loc := flavorBlockRe.FindStringIndex(text); loc != nil {
		block := braceBlock(text[loc[1]-1:])
		names := flavorKtsRe.FindAllStringSubmatch(block, -1)
		if len(names) == 0 {
			names = flavorGrRe.FindAllStringSubmatch(block, -1)
		}
		var all []string
		for _, n := range names {
			if n[1] != "productFlavors" {
				all = append(all, n[1])
			}
		}
		if len(all) > 0 {
			flavor = all[0]
			notes = append(notes, fmt.Sprintf("flavors: %s; using %q", strings.Join(all, ", "), flavor))
			if i := strings.Index(block, `"`+flavor+`"`); i >= 0 {
				if j := strings.Index(block[i:], "{"); j >= 0 {
					if m := suffixRe.FindStringSubmatch(braceBlock(block[i+j:])); m != nil {
						flavorSuffix = m[1]
					}
				}
			}
		}
	}

	gradle := "./gradlew"
	if !Exists(filepath.Join(root, gradleRoot, "gradlew")) {
		gradle = "gradle"
		notes = append(notes, "no gradlew next to the settings file; using gradle from PATH")
	}
	if gradleRoot != "." {
		gradle = "cd " + shellWord(gradleRoot) + " && " + gradle
		notes = append(notes, fmt.Sprintf("the Gradle build starts in %s", gradleRoot))
	}
	task := module + ":assemble" + upperFirst(flavor) + "Debug"
	artifactDir := filepath.Join(moduleDir, "build", "outputs", "apk")
	if flavor != "" {
		artifactDir = filepath.Join(artifactDir, flavor)
	}
	pl := &Platform{
		Build:      []string{gradle + " " + task},
		Artifact:   filepath.Join(artifactDir, "debug", "*.apk"),
		gradleRoot: gradleRoot,
	}
	if m := appIDRe.FindStringSubmatch(text); m != nil {
		pl.ID = m[1] + flavorSuffix
		notes = append(notes, "id comes from applicationId and the flavor suffix; the build reads the real one from the APK")
	}
	return pl, notes
}

// findGradleRoot returns the nearest directory from moduleDir up to the
// checkout root that has a settings file. Gradle module paths start there.
func findGradleRoot(root, moduleDir string) string {
	for dir := moduleDir; ; dir = filepath.Dir(dir) {
		if Exists(filepath.Join(root, dir, "settings.gradle.kts")) || Exists(filepath.Join(root, dir, "settings.gradle")) {
			return dir
		}
		if dir == "." || dir == string(filepath.Separator) {
			return "."
		}
	}
}

// braceBlock returns the text of the {...} block that starts at s[0].
func braceBlock(s string) string {
	depth := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[1:i]
			}
		}
	}
	return s
}

func detectIOS(root, container string, slow bool) (*Platform, []string) {
	flag := "-project"
	if strings.HasSuffix(container, ".xcworkspace") {
		flag = "-workspace"
	}
	notes := []string{fmt.Sprintf("found %s", container)}
	scheme := ""
	schemes := sharedSchemes(root, container)
	if len(schemes) == 0 {
		schemes = xcodeSchemes(root, flag, container)
	}
	schemes = rankSchemes(schemes, container)
	if len(schemes) > 0 {
		scheme = schemes[0]
		if len(schemes) > 1 {
			notes = append(notes, fmt.Sprintf("schemes: %s; using %q", strings.Join(schemes, ", "), scheme))
		}
	} else {
		scheme = strings.TrimSuffix(filepath.Base(container), filepath.Ext(container))
		notes = append(notes, fmt.Sprintf("xcodebuild -list gave no schemes; guessing %q", scheme))
	}

	product := "*.app"
	pl := &Platform{}
	if slow {
		if id, name := xcodeBuildSettings(root, flag, container, scheme); id != "" {
			pl.ID = id
			if name != "" {
				product = name
			}
		} else {
			notes = append(notes, "xcodebuild -showBuildSettings failed; set id by hand")
		}
	}
	pl.Build = []string{fmt.Sprintf(
		`xcodebuild %s %s -scheme %s -configuration Debug -destination "generic/platform=iOS Simulator" -derivedDataPath "{derived_data}" build`,
		flag, shellWord(container), shellWord(scheme))}
	pl.Artifact = filepath.Join("{derived_data}", "Build", "Products", "Debug-iphonesimulator", product)
	// A device build needs signing: the team of the Xcode project, and
	// -allowProvisioningUpdates lets Xcode create the profile.
	pl.Device = &DeviceBuild{
		Build: []string{fmt.Sprintf(
			`xcodebuild %s %s -scheme %s -configuration Debug -destination "generic/platform=iOS" -derivedDataPath "{derived_data}" -allowProvisioningUpdates build`,
			flag, shellWord(container), shellWord(scheme))},
		Artifact: filepath.Join("{derived_data}", "Build", "Products", "Debug-iphoneos", product),
	}
	return pl, notes
}

// sharedSchemes reads the schemes committed with the project. It is instant,
// and it leaves out the schemes of Swift packages that xcodebuild -list adds.
func sharedSchemes(root, container string) []string {
	files, _ := filepath.Glob(filepath.Join(root, container, "xcshareddata", "xcschemes", "*.xcscheme"))
	var out []string
	for _, f := range files {
		out = append(out, strings.TrimSuffix(filepath.Base(f), ".xcscheme"))
	}
	return out
}

// rankSchemes puts the likely app scheme first: the one named like the
// project, then any scheme that is not a test or an extension.
func rankSchemes(schemes []string, container string) []string {
	base := strings.TrimSuffix(filepath.Base(container), filepath.Ext(container))
	score := func(s string) int {
		l := strings.ToLower(s)
		switch {
		case s == base:
			return 0
		case strings.Contains(l, "test") || strings.Contains(l, "extension") || strings.Contains(l, "package"):
			return 3
		case strings.Contains(l, "app"):
			return 1
		}
		return 2
	}
	out := append([]string{}, schemes...)
	sort.SliceStable(out, func(i, j int) bool { return score(out[i]) < score(out[j]) })
	return out
}

func xcodeSchemes(root, flag, container string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xcrun", "xcodebuild", "-list", "-json", flag, container)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var v struct {
		Project   struct{ Schemes []string } `json:"project"`
		Workspace struct{ Schemes []string } `json:"workspace"`
	}
	if json.Unmarshal(out, &v) != nil {
		return nil
	}
	if len(v.Workspace.Schemes) > 0 {
		return v.Workspace.Schemes
	}
	return v.Project.Schemes
}

func xcodeBuildSettings(root, flag, container, scheme string) (id, product string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "xcrun", "xcodebuild", "-showBuildSettings", "-json", flag, container,
		"-scheme", scheme, "-configuration", "Debug", "-sdk", "iphonesimulator")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	var v []struct {
		BuildSettings map[string]string `json:"buildSettings"`
	}
	if json.Unmarshal(out, &v) != nil {
		return "", ""
	}
	for _, t := range v {
		s := t.BuildSettings
		if s["WRAPPER_EXTENSION"] == "app" || strings.HasSuffix(s["FULL_PRODUCT_NAME"], ".app") {
			return s["PRODUCT_BUNDLE_IDENTIFIER"], s["FULL_PRODUCT_NAME"]
		}
	}
	return "", ""
}

// generic directory names that say nothing about which app a module is.
var genericSegments = map[string]bool{
	"android": true, "ios": true, "app": true, "composeapp": true, "androidapp": true,
	"iosapp": true, "apps": true, "src": true, "mobile": true, "client": true, ".": true,
	"example": true, "examples": true, "sample": true, "samples": true, "demo": true,
}

// appKey names an app after its module path: the last segment that is not a
// generic word. apps/shop/android/composeApp gives "shop". It returns "" when
// every segment is generic.
func appKey(dir string) string {
	segs := strings.Split(filepath.ToSlash(dir), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if s := segs[i]; s != "" && !genericSegments[strings.ToLower(s)] {
			return slug(s)
		}
	}
	return ""
}

func uniqueKey(apps map[string]*App, key string) string {
	if key == "" {
		key = "app"
	}
	if _, taken := apps[key]; !taken {
		return key
	}
	for i := 2; ; i++ {
		k := fmt.Sprintf("%s-%d", key, i)
		if _, taken := apps[k]; !taken {
			return k
		}
	}
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.' || r == ' ':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	return out
}

// displayName turns a key like "sampleApp" or "sample-app" into "Sample App".
func displayName(key string) string {
	var words []string
	cur := []rune{}
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}
	for i, r := range key {
		switch {
		case r == '-' || r == '_' || r == '.':
			flush()
		case unicode.IsUpper(r) && i > 0:
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	for i, w := range words {
		words[i] = upperFirst(w)
	}
	return strings.Join(words, " ")
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func shellWord(s string) string {
	if strings.ContainsAny(s, " \t'\"$`\\") {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(s) + `"`
	}
	return s
}

// Render writes a detected project as a commented TOML file.
func (d *Detection) Render() string { return d.render(false) }

// RenderRepo writes the project as a .ujibox.toml for the repo: it has no
// [match] table, because the file lives in the checkout it describes.
func (d *Detection) RenderRepo() string { return d.render(true) }

func (d *Detection) render(repo bool) string {
	p := d.Project
	var b strings.Builder
	if repo {
		b.WriteString("# ujibox project file, shared by everyone who clones this repo.\n")
		b.WriteString("# `uji config project --repo` wrote it from what it found in the checkout.\n")
		b.WriteString("# A project file in ~/.config/ujibox/projects overrides its keys on one machine.\n")
	} else {
		b.WriteString("# ujibox project file. `uji config project` wrote it from what it found in the checkout.\n")
	}
	b.WriteString("# Edit it freely; `uji config show` prints the result with {variables} filled in.\n")
	b.WriteString("#\n")
	b.WriteString("# Variables in run, build and artifact values (also exported as UJI_<NAME>):\n")
	b.WriteString("#   {root} the checkout, {main_root} the main checkout of the repo,\n")
	b.WriteString("#   {derived_data} a per-checkout cache dir, {branch}, {worktree}, {app}, {platform}\n\n")
	fmt.Fprintf(&b, "name = %q\n", p.Name)
	if p.DefaultApp != "" {
		fmt.Fprintf(&b, "default_app = %q\n", p.DefaultApp)
	}
	if !repo {
		writeMatch(&b, p.Match)
	}

	for _, k := range p.AppKeys() {
		a := p.Apps[k]
		fmt.Fprintf(&b, "\n[apps.%s]\n", tomlKey(k))
		fmt.Fprintf(&b, "display = %q\n", a.Display)
		b.WriteString("# env = { JAVA_HOME = \"/path/to/jdk\" }\n")
		for _, name := range a.Platforms() {
			pl := a.Platform(name)
			fmt.Fprintf(&b, "\n[apps.%s.%s]\n", tomlKey(k), name)
			for _, n := range d.Notes[k+"."+name] {
				fmt.Fprintf(&b, "# %s\n", n)
			}
			if pl.ID != "" {
				fmt.Fprintf(&b, "id = %q\n", pl.ID)
			} else {
				b.WriteString("id = \"\"   # application id / bundle id, used by launch and uninstall\n")
			}
			b.WriteString("build = [\n")
			for _, c := range pl.Build {
				fmt.Fprintf(&b, "  %s,\n", tomlString(c))
			}
			b.WriteString("]\n")
			if name == Android {
				b.WriteString("# A wrapper works too: build = [\"scripts/my_gradle.py :app:assembleDevDebug\"]\n")
			} else {
				b.WriteString("# KMP projects often need ARCHS=arm64 for simulator builds; add it to the xcodebuild line.\n")
			}
			fmt.Fprintf(&b, "artifact = %s\n", tomlString(pl.Artifact))
			for _, s := range pl.Setup {
				fmt.Fprintf(&b, "\n[[apps.%s.%s.setup]]\nrun = %s\n", tomlKey(k), name, tomlString(s.Run))
				if s.Creates != "" {
					fmt.Fprintf(&b, "creates = %q\n", s.Creates)
				}
			}
			for _, r := range pl.Retry {
				fmt.Fprintf(&b, "\n[[apps.%s.%s.retry]]\nmatch = %s\nthen = %s\n", tomlKey(k), name, tomlString(r.Match), tomlString(r.Then))
			}
			if name == IOS && pl.Device != nil {
				fmt.Fprintf(&b, "\n# The build for physical devices. It needs code signing: if the Xcode project\n")
				fmt.Fprintf(&b, "# has no team, add DEVELOPMENT_TEAM=<team id> to the xcodebuild line.\n")
				fmt.Fprintf(&b, "[apps.%s.ios.device]\nbuild = [\n", tomlKey(k))
				for _, c := range pl.Device.Build {
					fmt.Fprintf(&b, "  %s,\n", tomlString(c))
				}
				fmt.Fprintf(&b, "]\nartifact = %s\n", tomlString(pl.Device.Artifact))
			}
			if name == IOS {
				fmt.Fprintf(&b, "\n# Steps that run before every build, for example a project generator:\n")
				fmt.Fprintf(&b, "# [[apps.%s.ios.setup]]\n# run = \"./scripts/generate-project.sh\"\n", tomlKey(k))
				fmt.Fprintf(&b, "\n# Steps for `uji build --clean`:\n")
				fmt.Fprintf(&b, "# [[apps.%s.ios.clean]]\n# run = \"rm -rf {derived_data}/SourcePackages/checkouts\"\n", tomlKey(k))
			}
		}
	}
	return b.String()
}

func writeMatch(b *strings.Builder, m Match) {
	b.WriteString("\n[match]\n")
	if m.Remote != "" {
		fmt.Fprintf(b, "remote = %q\n", m.Remote)
		b.WriteString("# paths = [\"~/code/my-repo*\"]   # match checkout roots instead of, or as well as, the remote\n")
	} else if len(m.Paths) > 0 {
		fmt.Fprintf(b, "paths = [%q]\n", m.Paths[0])
	}
}

// RenderOverride writes a machine project file that overrides the repo file
// of a checkout. It has only the [match] table and commented examples.
func RenderOverride(repo *Project, m Match) string {
	var b strings.Builder
	b.WriteString("# ujibox project file for this machine only. It overrides the keys of the\n")
	fmt.Fprintf(&b, "# shared %s of the repo: set only what differs here.\n", RepoFileName)
	b.WriteString("# `uji config show` prints the result.\n")
	writeMatch(&b, m)
	key := repo.DefaultApp
	if key == "" {
		if keys := repo.AppKeys(); len(keys) > 0 {
			key = keys[0]
		}
	}
	if key == "" {
		key = "<app>"
	}
	k := tomlKey(key)
	b.WriteString("\n# Examples:\n")
	fmt.Fprintf(&b, "# [apps.%s]\n# env = { JAVA_HOME = \"/path/to/jdk\" }\n#\n", k)
	fmt.Fprintf(&b, "# [apps.%s.ios.device]\n", k)
	b.WriteString("# build = ['xcodebuild … DEVELOPMENT_TEAM=<your team id> build']\n")
	return b.String()
}

func tomlKey(k string) string { return quoteKey(k) }

// tomlString writes a literal string when it can, so regexes and quotes stay readable.
func tomlString(s string) string {
	if !strings.Contains(s, "'") && !strings.Contains(s, "\n") {
		return "'" + s + "'"
	}
	return fmt.Sprintf("%q", s)
}
