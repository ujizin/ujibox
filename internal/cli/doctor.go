package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdDoctor(get func() *app) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Check the tools, the config and the store",
		GroupID: "store",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			t := a.mgr.Tools
			problems := 0
			check := func(ok bool, name, found, fix string) {
				if ok {
					fmt.Printf("%s %-14s %s\n", ui.Green("✔"), name, found)
					return
				}
				problems++
				fmt.Printf("%s %-14s %s\n", ui.Red("✘"), name, fix)
			}
			optional := func(ok bool, name, found, why string) {
				if ok {
					fmt.Printf("%s %-14s %s\n", ui.Green("✔"), name, found)
					return
				}
				fmt.Printf("%s %-14s %s\n", ui.Yellow("!"), name, why)
			}

			fmt.Println(ui.Bold("Tools"))
			check(t.ADB != "", "adb", paths.Pretty(t.ADB), "not found: install Android SDK platform-tools")
			optional(t.Emulator != "", "emulator", paths.Pretty(t.Emulator), "not found: uji cannot boot AVDs")
			optional(t.AAPT2 != "", "aapt2", paths.Pretty(t.AAPT2), "not found: builds will not record the APK version")
			optional(t.Scrcpy != "", "scrcpy", paths.Pretty(t.Scrcpy), "not found: `brew install scrcpy` to record Android")
			check(t.HasSimctl(), "Xcode", t.DeveloperDir, "simctl not found: install Xcode or set developer_dir")
			optional(t.HasDevicectl(), "devicectl", "physical iOS devices", "not found: physical iOS devices need Xcode 15 or later")
			chrome := device.ChromePath()
			optional(chrome != "", "Chrome", paths.Pretty(chrome), "not found: `uji launch web` serves the app, but screenshot and record need its Chrome window")
			if ok, known := device.ScreenCaptureAllowed(); known {
				optional(ok, "screen record", "the terminal has the Screen Recording permission",
					"the terminal has no Screen Recording permission, so uji cannot capture desktop and web windows. "+
						"Turn it on for your terminal app in System Settings > Privacy & Security > Screen & System Audio Recording, then restart the terminal")
			}
			if sel, err := exec.Command("xcode-select", "-p").Output(); err == nil && t.DeveloperDir != "" {
				if s := strings.TrimSpace(string(sel)); s != t.DeveloperDir {
					ui.Info(os.Stdout, "%s", ui.Dim("xcode-select points at "+s+"; uji uses "+t.DeveloperDir))
				}
			}

			fmt.Println(ui.Bold("\nConfig"))
			if _, err := config.LoadGlobal(); err != nil {
				check(false, "global", "", err.Error())
			} else {
				check(true, "global", paths.Pretty(config.GlobalPath()), "")
			}
			projects, errs := config.LoadProjects()
			for _, e := range errs {
				check(false, "project file", "", e.Error())
			}
			fmt.Printf("  %-14s %d in %s\n", "projects", len(projects), paths.Pretty(paths.Projects()))

			if wd, err := os.Getwd(); err == nil {
				if g, ok := gitinfo.Read(a.ctx, wd); ok {
					p, _ := config.Find(g)
					switch {
					case p == nil:
						optional(false, "this checkout", "", "no project matches and none was detected; run `uji config project` in an app checkout")
					case p.Detected:
						optional(false, "this checkout", "", "no project file; uji detects one ("+strings.Join(p.AppKeys(), ", ")+"). `uji config project` saves it")
					default:
						check(true, "this checkout", p.Name+" ("+paths.Pretty(p.File)+")", "")
					}
				}
			}

			fmt.Println(ui.Bold("\nStore"))
			list := a.store.List(store.Filter{})
			var size int64
			for _, m := range list {
				size += m.Artifact.Size
			}
			fmt.Printf("  %-14s %s\n", "builds", paths.Pretty(a.store.Root))
			fmt.Printf("  %-14s %d builds, %s, kept %d days (newest %d per branch)\n", "", len(list), ui.Size(size), a.global.RetentionDays, a.global.KeepLast)
			fmt.Printf("  %-14s %s\n", "cache", paths.Pretty(paths.Cache()))

			if problems > 0 {
				return &quietError{fmt.Sprintf("%d problems", problems)}
			}
			return nil
		},
	}
}
