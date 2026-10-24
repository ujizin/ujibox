package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdConfig(get func() *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Write, show and edit the project and global config",
		Long: `Project files live in ` + paths.Pretty(paths.Projects()) + `, one per project, on this machine
only. A file matches a checkout by its git remote or by path, so every
worktree of a repo uses the same file.

A repo can also share its config in ` + config.RepoFileName + ` at the root of the checkout.
A project file on this machine overrides its keys.

The global config is ` + paths.Pretty(config.GlobalPath()) + `.`,
		GroupID: "store",
	}

	var force, print, repo bool
	project := &cobra.Command{
		Use:   "project",
		Short: "Detect the project of this checkout and write its project file",
		Long: `Look at the checkout for Gradle application modules and Xcode projects,
then write a project file with what uji found. Each value has a comment that
says where it came from. Edit the file after: build commands, setup steps
and retry rules are yours to change.

By default the file goes to ` + paths.Pretty(paths.Projects()) + `, on this machine only.
With --repo, uji writes ` + config.RepoFileName + ` in the root of the checkout; commit it to
share the config with everyone who clones the repo.

If the checkout already has ` + config.RepoFileName + `, uji writes a small file for this
machine that overrides only the keys you set in it.`,
		Example: `  uji config project
  uji config project --repo
  uji config project --print`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			g, err := a.checkout()
			if err != nil {
				return err
			}
			shared, err := config.LoadRepoFile(g)
			if err != nil && !repo {
				return err
			}
			if shared != nil && !repo && !print {
				return writeOverride(g, shared, force)
			}

			ui.Info(os.Stderr, "looking at %s…", paths.Pretty(g.Root))
			det := config.DetectVerbose(g)
			if det == nil {
				return errors.New("found no Gradle application module and no Xcode project; write the file by hand (see `uji help config`)")
			}
			text := det.Render()
			if repo {
				text = det.RenderRepo()
			}
			if print {
				fmt.Print(text)
				return nil
			}
			file := filepath.Join(paths.Projects(), det.Project.Name+".toml")
			if repo {
				file = config.RepoFilePath(g)
			} else {
				projects, _ := config.LoadProjects()
				for _, p := range projects {
					if p.Matches(g) && p.File != file {
						file = p.File
					}
				}
			}
			if _, err := os.Stat(file); err == nil && !force {
				return fmt.Errorf("%s exists; edit it with `uji config edit`, or pass --force to replace it", paths.Pretty(file))
			}
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
				return err
			}
			if _, err := config.LoadProjectFile(file); err != nil {
				ui.Warn(os.Stderr, "the file does not load yet: %v", err)
			}
			ui.OK(os.Stdout, "wrote %s", paths.Pretty(file))
			for _, k := range det.Project.AppKeys() {
				ap := det.Project.Apps[k]
				ui.Info(os.Stdout, "app %s: %s", ui.Bold(ap.Name()), strings.Join(ap.Platforms(), ", "))
			}
			if repo {
				fmt.Println(ui.Dim("Check the build commands, then commit " + config.RepoFileName + " to share it."))
			} else {
				fmt.Println(ui.Dim("Check the build commands, then run `uji build`. `uji config edit` opens the file."))
			}
			return nil
		},
	}
	project.Flags().BoolVarP(&force, "force", "f", false, "replace an existing project file")
	project.Flags().BoolVar(&print, "print", false, "print the file instead of writing it")
	project.Flags().BoolVar(&repo, "repo", false, "write "+config.RepoFileName+" in the checkout, to share it in the repo")

	edit := &cobra.Command{
		Use:   "edit [global|repo]",
		Short: "Open the project file of this checkout, the repo file, or the global config, in $EDITOR",
		Long: `With no argument, open the project file of this checkout: the file on this
machine, or ` + config.RepoFileName + ` when there is no file on this machine.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			file := config.GlobalPath()
			switch {
			case len(args) == 0:
				_, p, _, err := a.project("")
				if p == nil {
					return err
				}
				if p.EditFile() == "" {
					return errors.New("no project file matches this checkout; run `uji config project` first")
				}
				file = p.EditFile()
			case args[0] == "repo":
				g, err := a.checkout()
				if err != nil {
					return err
				}
				file = config.RepoFilePath(g)
				if _, err := os.Stat(file); err != nil {
					return fmt.Errorf("this checkout has no %s; run `uji config project --repo` first", config.RepoFileName)
				}
			case args[0] != "global":
				return fmt.Errorf("unknown target %q; use `uji config edit`, `uji config edit repo` or `uji config edit global`", args[0])
			}
			return openEditor(file)
		},
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Print the project config of this checkout with the variables filled in",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			g, p, _, err := a.project("")
			if p == nil {
				return err
			}
			src := p.Sources()
			fmt.Printf("%s %s\n%s %s\n%s %s @ %s\n", ui.Dim("project "), p.Name, ui.Dim("source  "), src,
				ui.Dim("checkout"), paths.Pretty(g.Root), g.Group())
			for _, k := range p.AppKeys() {
				ap := p.Apps[k]
				fmt.Printf("\n%s %s\n", ui.Bold(ap.Name()), ui.Dim("["+k+"] store id "+p.StoreID(ap)))
				for _, name := range ap.Platforms() {
					pl := ap.Platform(name)
					v := config.NewVars(g, ap, name)
					fmt.Printf("  %s\n", name)
					fmt.Printf("    id        %s\n", pl.ID)
					for _, s := range append(append([]config.Step{}, ap.Setup...), pl.Setup...) {
						note := ""
						if s.Creates != "" {
							note = ui.Dim(" (skipped when " + v.Expand(s.Creates) + " exists)")
						}
						fmt.Printf("    setup     %s%s\n", v.Expand(s.Run), note)
					}
					for _, b := range pl.Build {
						fmt.Printf("    build     %s\n", v.Expand(b))
					}
					fmt.Printf("    artifact  %s\n", v.Expand(pl.Artifact))
					for _, s := range append(append([]config.Step{}, ap.Clean...), pl.Clean...) {
						fmt.Printf("    clean     %s\n", v.Expand(s.Run))
					}
					for _, r := range append(append([]config.Retry{}, ap.Retry...), pl.Retry...) {
						fmt.Printf("    retry     /%s/ → %s\n", r.Match, v.Expand(r.Then))
					}
					if dev, err := pl.ForDevice(); err == nil {
						fmt.Printf("  %s\n", "ios device")
						for _, b := range dev.Build {
							fmt.Printf("    build     %s\n", v.Expand(b))
						}
						fmt.Printf("    artifact  %s\n", v.Expand(dev.Artifact))
					}
				}
			}
			return nil
		},
	}

	path := &cobra.Command{
		Use:   "path [global|repo]",
		Short: "Print the path of the project file of this checkout, the repo file, or the global config",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "global" {
				fmt.Println(config.GlobalPath())
				return nil
			}
			if len(args) == 1 && args[0] == "repo" {
				g, err := get().checkout()
				if err != nil {
					return err
				}
				fmt.Println(config.RepoFilePath(g))
				return nil
			}
			_, p, _, err := get().project("")
			if p == nil {
				return err
			}
			if p.EditFile() == "" {
				return errors.New("no project file matches this checkout; run `uji config project`")
			}
			fmt.Println(p.EditFile())
			return nil
		},
	}

	alias := &cobra.Command{
		Use:   "alias <name> <device>",
		Short: "Give a device a short name, for example `uji config alias fold Pixel_9_Pro_Fold`",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, target := args[0], args[1]
			switch strings.ToLower(name) {
			case "android", "ios", "all":
				return fmt.Errorf("%q is a platform word; pick another alias", name)
			}
			a := get()
			devs, _ := a.mgr.List(a.ctx, device.AllPlatforms)
			known := false
			for _, d := range devs {
				if d.ID == target || d.Name == target || d.AVD == target || d.Ref() == target {
					known = true
				}
			}
			if !known {
				ui.Warn(os.Stderr, "no device is called %q right now; saving the alias anyway", target)
			}
			if err := config.SetAlias(name, target); err != nil {
				return err
			}
			ui.OK(os.Stdout, "%s → %s", name, target)
			return nil
		},
	}

	autoSync := &cobra.Command{
		Use:   "auto-sync [on|off]",
		Short: "Show, or turn on or off, the sync of the ujibox app after each store change",
		Long: `After a command adds or removes a stored build, uji syncs the ujibox app on
each running device that has it installed. Auto-sync is on by default.

With no argument, uji prints the current state. --no-sync turns it off for
one command.`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"on", "off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			if len(args) == 0 {
				state := "off"
				if a.global.AutoSyncOn() {
					state = "on"
				}
				fmt.Println(state)
				return nil
			}
			var on bool
			switch strings.ToLower(args[0]) {
			case "on", "true", "yes":
				on = true
			case "off", "false", "no":
			default:
				return fmt.Errorf("%q is not on or off", args[0])
			}
			if err := config.SetAutoSync(on); err != nil {
				return err
			}
			state := "off"
			if on {
				state = "on"
			}
			ui.OK(os.Stdout, "auto-sync %s", state)
			return nil
		},
	}

	c.AddCommand(project, edit, show, path, alias, autoSync)
	return c
}

// writeOverride writes a machine file that overrides the repo file of the
// checkout. It has only a [match] table and commented examples.
func writeOverride(g gitinfo.Info, shared *config.Project, force bool) error {
	m := config.Match{Remote: g.Remote}
	if g.Remote == "" {
		m.Paths = []string{g.MainRoot}
	}
	file := filepath.Join(paths.Projects(), shared.Name+".toml")
	projects, _ := config.LoadProjects()
	for _, p := range projects {
		if p.Matches(g) {
			file = p.File
		}
	}
	if _, err := os.Stat(file); err == nil && !force {
		return fmt.Errorf("%s exists and overrides %s; edit it with `uji config edit`, or pass --force to replace it",
			paths.Pretty(file), config.RepoFileName)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(config.RenderOverride(shared, m)), 0o644); err != nil {
		return err
	}
	ui.OK(os.Stdout, "wrote %s", paths.Pretty(file))
	fmt.Println(ui.Dim("This checkout shares its config in " + config.RepoFileName + ". The new file is for this machine:\nset in it only the keys that differ here. Use --repo to write " + config.RepoFileName + " again."))
	return nil
}

func openEditor(file string) error {
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(globalTemplate), 0o644); err != nil {
			return err
		}
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command("/bin/sh", "-c", editor+` "$1"`, "sh", file)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

const globalTemplate = `# ujibox global config. Every key is optional.

# retention_days = 30     # how long an unpinned build stays in the store
# keep_last = 3           # builds of each branch and platform that never expire
# developer_dir = "/Applications/Xcode.app/Contents/Developer"
# android_sdk = "~/Library/Android/sdk"
# sync_package = "io.ujibox.app"     # Android package of the ujibox app
# sync_bundle_id = "io.ujibox.app"   # bundle id of the ujibox app on simulators
# auto_sync = true        # sync the ujibox app on running devices after each store change

# [aliases]
# fold = "Pixel_9_Pro_Fold"
`
