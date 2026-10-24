package builder

import (
	"strings"
)

// Summarize picks the lines of a build log that explain a failure: Gradle's
// "What went wrong" block, the xcodebuild error lines, or the log tail.
func Summarize(log string) string {
	lines := strings.Split(log, "\n")

	var block []string
	in := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "* What went wrong:"):
			in = true
			block = append(block, "")
		case strings.HasPrefix(t, "* Try:"), strings.HasPrefix(t, "* Get more help"):
			in = false
		case in && t != "":
			block = append(block, t)
		}
	}
	if s := strings.TrimSpace(strings.Join(block, "\n")); s != "" {
		return limit(s, 30)
	}

	// A failed Xcode script phase prints its reason above this line, without "error:".
	for i, l := range lines {
		if strings.Contains(l, "Command PhaseScriptExecution failed") {
			var out []string
			for j := i - 1; j >= 0 && len(out) < 12; j-- {
				t := strings.TrimRight(lines[j], " \r")
				if strings.HasPrefix(t, "PhaseScriptExecution ") {
					break
				}
				if strings.TrimSpace(t) != "" && !strings.HasPrefix(strings.TrimSpace(t), "/bin/sh -c") && !strings.HasPrefix(strings.TrimSpace(t), "cd ") {
					out = append([]string{t}, out...)
				}
			}
			out = append(out, strings.TrimSpace(l))
			return strings.Join(out, "\n")
		}
	}

	var errs []string
	seen := map[string]bool{}
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if (strings.Contains(t, "error:") || strings.HasPrefix(t, "e: ") || strings.Contains(t, "** BUILD FAILED **")) && !seen[t] {
			seen[t] = true
			errs = append(errs, t)
		}
	}
	if len(errs) > 0 {
		return limit(strings.Join(errs, "\n"), 20)
	}

	var tail []string
	for i := len(lines) - 1; i >= 0 && len(tail) < 15; i-- {
		if t := strings.TrimRight(lines[i], " \r"); t != "" {
			tail = append([]string{t}, tail...)
		}
	}
	return strings.Join(tail, "\n")
}

func limit(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n…"
}

// hintFor maps known failure text to the step that fixes it.
func hintFor(log string) string {
	switch {
	case strings.Contains(log, "is not found in cache") || strings.Contains(log, "Failed to build cache"):
		return "the Kotlin/Native cache is broken; run the build again with --clean (add a clean step to the project if there is none)"
	case strings.Contains(log, "Package.swift doesn't exist"), strings.Contains(log, "Could not resolve package dependencies"):
		return "the SPM checkouts are empty; run the build again with --clean"
	case strings.Contains(log, "The Xcode build system has crashed"):
		return "Xcode crashed during the build; run it again, and with --clean if it fails again"
	case strings.Contains(log, "SDK location not found"):
		return "Gradle cannot find the Android SDK; add a setup step that writes local.properties, or set ANDROID_HOME"
	case strings.Contains(log, "requires a development team"), strings.Contains(log, "No profiles for"),
		strings.Contains(log, "No signing certificate"), strings.Contains(log, "No Account for Team"):
		return "a device build needs code signing: sign in to Xcode with your Apple ID (Settings > Accounts), then set the team in the Xcode project or add DEVELOPMENT_TEAM=<team id> to the device build command"
	case strings.Contains(log, "No such module"), strings.Contains(log, "does not contain a scheme"):
		return "the Xcode project may need to be generated first; add the generator as a setup step"
	}
	return ""
}
