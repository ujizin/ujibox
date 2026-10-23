package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/builder"
	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

// buildFlags are shared by build and install.
type buildFlags struct {
	app     string
	clean   bool
	verbose bool
	platformFlags
}

func (b *buildFlags) add(c *cobra.Command) {
	c.Flags().StringVar(&b.app, "app", "", "app of the project to build, when it has more than one")
	c.Flags().BoolVarP(&b.clean, "clean", "C", false, "run the project's clean steps first")
	c.Flags().BoolVarP(&b.verbose, "verbose", "V", false, "print the full build output")
	b.platformFlags.add(c)
}

func cmdBuild(get func() *app) *cobra.Command {
	var bf buildFlags
	c := &cobra.Command{
		Use:   "build [android|ios|device...]",
		Short: "Build the app and keep it in the store, without installing it",
		Long: `Build the app of the current checkout and keep the artifact in the store.

With no selector, uji builds every platform the app has. A platform word builds
only that platform; a device name builds the platform of that device.`,
		Example: `  uji build
  uji build android
  uji build ios --clean`,
		GroupID: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			g, p, ap, err := a.project(bf.app)
			if err != nil {
				return err
			}
			sel := device.ParseSelectors(args, bf.android, bf.ios)
			platforms := sel.PlatformsOr(ap.Platforms())
			if len(sel.Names) > 0 {
				devs, err := a.mgr.Resolve(a.ctx, sel, ap.Platforms())
				if err != nil {
					return err
				}
				platforms = devicePlatforms(devs)
			}
			platforms, err = supported(ap, platforms)
			if err != nil {
				return err
			}
			announce(g, p, ap)
			var built []string
			defer func() { a.autoSync(built...) }()
			for _, pl := range platforms {
				if _, err := a.build(g, p, ap, pl, bf); err != nil {
					return err
				}
				built = append(built, pl)
			}
			a.store.MaybePrune(a.policy())
			return nil
		},
	}
	bf.add(c)
	return c
}

func announce(g gitinfo.Info, p *config.Project, ap *config.App) {
	src := "project " + p.Name
	if p.Detected {
		src = "detected project (no project file; `uji config project` writes one)"
	}
	dirty := ""
	if g.Dirty {
		dirty = ui.Yellow(" dirty")
	}
	ui.Info(os.Stderr, "%s · %s · %s@%s%s", ui.Bold(ap.Name()), src, g.Group(), g.ShortSHA(), dirty)
}

func (a *app) build(g gitinfo.Info, p *config.Project, ap *config.App, platform string, bf buildFlags) (*store.Manifest, error) {
	m, err := builder.Run(a.ctx, builder.Request{
		Project: p, App: ap, Platform: platform, Git: g, Store: a.store, Tools: a.mgr.Tools,
		Clean: bf.clean, Verbose: bf.verbose, Out: os.Stderr,
	})
	if err != nil {
		return nil, err
	}
	ui.OK(os.Stderr, "%s %s %s · %s · %s · %s", ap.Name(), platform, versionText(m), ui.Size(m.Artifact.Size),
		ui.Elapsed(msDuration(m.Build.DurationMS)), ui.Dim("build "+m.ShortID()))
	ui.Info(os.Stderr, "%s", ui.Dim(paths.Pretty(m.ArtifactPath())))
	return m, nil
}

func versionText(m *store.Manifest) string {
	switch {
	case m.Version.Name != "" && m.Version.Code != "":
		return m.Version.Name + " (" + m.Version.Code + ")"
	case m.Version.Name != "":
		return m.Version.Name
	}
	return m.PackageID
}

func devicePlatforms(devs []device.Device) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range device.AllPlatforms {
		for _, d := range devs {
			if d.Platform == p && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// supported keeps the platforms the app can build. None left is an error.
func supported(ap *config.App, platforms []string) ([]string, error) {
	var out, missing []string
	for _, p := range platforms {
		if ap.Platform(p) != nil {
			out = append(out, p)
		} else {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		ui.Warn(os.Stderr, "%s has no %s config; skipping %s", ap.Name(), strings.Join(missing, " or "), strings.Join(missing, ", "))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has nothing to build for %s", ap.Name(), strings.Join(platforms, ", "))
	}
	return out, nil
}

func cmdInstall(get func() *app) *cobra.Command {
	var bf buildFlags
	var last, noLaunch, anyBranch bool
	var ref string
	c := &cobra.Command{
		Use:   "install [android|ios|device...]",
		Short: "Build the app, store it, and install it on devices",
		Long: `Build the app, keep the build in the store, then install it on every
selected device in parallel and launch it.

With --last, uji skips the build and installs the newest stored build of this
branch. --build picks another stored build: a number from ` + "`uji builds`" + `, a
build id, a commit sha, or a branch name.

A named AVD or simulator that is off boots first. With no selector, uji uses
every running device and never boots one.`,
		Example: `  uji install
  uji install android
  uji install ios "iPhone 17 Pro"
  uji install fold --last
  uji install -b feature/login`,
		GroupID: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			g, p, ap, err := a.project(bf.app)
			if err != nil {
				if ref == "" || last {
					return err
				}
				// A build id or sha names one build; it needs no project.
				return a.installStored(ref, args, bf, noLaunch)
			}
			sel := device.ParseSelectors(args, bf.android, bf.ios)
			devs, err := a.mgr.Resolve(a.ctx, sel, ap.Platforms())
			if err != nil {
				return err
			}
			if _, err := supported(ap, devicePlatforms(devs)); err != nil {
				return err
			}
			if last && ref == "" {
				ref = "last"
			}

			// Pick or build one artifact per platform before touching any device.
			builds := map[string]*store.Manifest{}
			buildErrs := map[string]error{}
			announce(g, p, ap)
			for _, pl := range devicePlatforms(devs) {
				if ap.Platform(pl) == nil {
					continue
				}
				var m *store.Manifest
				if ref != "" {
					m, err = a.pick(g, p, ap, pl, ref, anyBranch)
				} else {
					m, err = a.build(g, p, ap, pl, bf)
				}
				if err != nil {
					if a.ctx.Err() != nil {
						return err
					}
					buildErrs[pl] = err
					var be *builder.Error
					if errors.As(err, &be) || ref != "" {
						ui.Fail(os.Stderr, "%v", err)
					}
					continue
				}
				builds[pl] = m
			}
			if len(builds) == 0 {
				return &quietError{"nothing to install"}
			}

			devs, err = a.mgr.EnsureReady(a.ctx, devs, func(s string) { ui.Info(os.Stderr, "%s", s) })
			if err != nil {
				return err
			}
			for _, m := range builds {
				if l := a.store.Use(m); l != nil {
					defer l.Unlock()
				}
			}
			err = a.installOn(devs, builds, noLaunch, func(platform string) error {
				if e := buildErrs[platform]; e != nil {
					return fmt.Errorf("no %s build: %s", platform, firstLine(e.Error()))
				}
				return fmt.Errorf("%s has no %s config", ap.Name(), platform)
			})
			if ref == "" {
				a.store.MaybePrune(a.policy())
				var built []string
				for pl := range builds {
					built = append(built, pl)
				}
				sort.Strings(built)
				a.autoSync(built...)
			}
			return err
		},
	}
	bf.add(c)
	c.Flags().BoolVarP(&last, "last", "L", false, "install the newest stored build of this branch; skip the build")
	c.Flags().StringVarP(&ref, "build", "b", "", "install a stored build: N, build id, commit sha or branch")
	c.Flags().BoolVar(&anyBranch, "any-branch", false, "with --last, take the newest build of any branch")
	c.Flags().BoolVar(&noLaunch, "no-launch", false, "install without launching the app")
	return c
}

// pick finds a stored build and warns when it differs from the checkout.
func (a *app) pick(g gitinfo.Info, p *config.Project, ap *config.App, platform, ref string, anyBranch bool) (*store.Manifest, error) {
	scope := store.Filter{AppID: p.StoreID(ap), Platform: platform, Group: g.Group()}
	if anyBranch {
		scope.Group = ""
	}
	m, err := a.store.Find(ref, scope)
	if errors.Is(err, store.ErrNoBuild) && ref == "last" && !anyBranch {
		others := a.store.List(store.Filter{AppID: scope.AppID, Platform: platform})
		msg := fmt.Sprintf("no stored %s build of %s for %s", platform, ap.Name(), g.Group())
		if len(others) > 0 {
			msg += fmt.Sprintf("; the newest is from %s (%s); use --any-branch or -b %q", others[0].Group(), ui.Ago(others[0].CreatedAt), others[0].Group())
		} else {
			msg += "; run `uji build` first"
		}
		return nil, errors.New(msg)
	}
	if err != nil {
		return nil, err
	}
	ui.Info(os.Stderr, "%s: stored build %s · %s@%s · %s", platform, m.ShortID(), m.Group(), m.ShortSHA(), ui.Ago(m.CreatedAt))
	if m.Group() == g.Group() && m.Git.SHA != g.SHA {
		ui.Warn(os.Stderr, "%s: this build is from %s, but HEAD is %s", platform, m.ShortSHA(), g.ShortSHA())
	}
	if m.Git.Dirty {
		ui.Warn(os.Stderr, "%s: this build was made from uncommitted changes", platform)
	}
	return m, nil
}

func cmdSetup(get func() *app) *cobra.Command {
	var appKey string
	c := &cobra.Command{
		Use:   "setup [android|ios]",
		Short: "Run the project's setup steps without building",
		Long: `Run the setup steps of the project file, for example the copy of
local.properties into a new worktree. A step with "creates" is skipped when
its file exists. ` + "`uji build`" + ` runs these steps too.`,
		GroupID: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			g, p, ap, err := a.project(appKey)
			if err != nil {
				return err
			}
			sel := device.ParseSelectors(args, false, false)
			platforms, err := supported(ap, sel.PlatformsOr(ap.Platforms()))
			if err != nil {
				return err
			}
			for _, pl := range platforms {
				if err := builder.Setup(a.ctx, builder.Request{Project: p, App: ap, Platform: pl, Git: g, Tools: a.mgr.Tools, Out: os.Stderr}); err != nil {
					return err
				}
			}
			ui.OK(os.Stderr, "setup done for %s", strings.Join(platforms, " and "))
			return nil
		},
	}
	c.Flags().StringVar(&appKey, "app", "", "app of the project")
	return c
}

func msDuration(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }

// installOn installs one build per platform on each device, in parallel.
func (a *app) installOn(devs []device.Device, builds map[string]*store.Manifest, noLaunch bool, missing func(string) error) error {
	return forEach(devs, func(d device.Device) (string, error) {
		m := builds[d.Platform]
		if m == nil {
			return "", missing(d.Platform)
		}
		if err := a.mgr.Install(a.ctx, d, m.ArtifactPath()); err != nil {
			return "", err
		}
		note := "installed " + versionText(m)
		if !noLaunch && m.PackageID != "" {
			if err := a.mgr.Launch(a.ctx, d, m.PackageID); err != nil {
				return note + ui.Yellow(" (launch failed: "+firstLine(err.Error())+")"), nil
			}
			note += ", launched"
		}
		return note, nil
	})
}

// installStored installs one stored build, found by id or sha in every app.
func (a *app) installStored(ref string, args []string, bf buildFlags, noLaunch bool) error {
	m, err := a.store.Find(ref, store.Filter{})
	if err != nil {
		return err
	}
	ui.Info(os.Stderr, "%s %s: stored build %s · %s@%s · %s", m.App.Name, m.Platform, m.ShortID(), m.Group(), m.ShortSHA(), ui.Ago(m.CreatedAt))
	sel := device.ParseSelectors(args, bf.android, bf.ios)
	devs, err := a.mgr.Resolve(a.ctx, sel, []string{m.Platform})
	if err != nil {
		return err
	}
	devs, err = a.mgr.EnsureReady(a.ctx, devs, func(s string) { ui.Info(os.Stderr, "%s", s) })
	if err != nil {
		return err
	}
	if l := a.store.Use(m); l != nil {
		defer l.Unlock()
	}
	return a.installOn(devs, map[string]*store.Manifest{m.Platform: m}, noLaunch, func(platform string) error {
		return fmt.Errorf("build %s is for %s", m.ShortID(), m.Platform)
	})
}
