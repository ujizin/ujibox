// Package cli defines the uji commands.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/builder"
	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/tui"
	"github.com/ujizin/ujibox/internal/ui"
)

// Version is set at build time with -ldflags "-X .../cli.Version=x.y.z".
var Version = "dev"

// app is the state shared by the commands of one run.
type app struct {
	ctx    context.Context
	global config.Global
	mgr    *device.Manager
	store  *store.Store
}

func newApp(ctx context.Context) *app {
	g, err := config.LoadGlobal()
	if err != nil {
		ui.Warn(os.Stderr, "ignoring the global config: %v", err)
	}
	return &app{ctx: ctx, global: g, mgr: device.NewManager(g), store: store.Open(paths.Data())}
}

func (a *app) policy() store.Policy {
	return store.Policy{MaxAge: time.Duration(a.global.RetentionDays) * 24 * time.Hour, KeepLast: a.global.KeepLast}
}

// checkout returns the git checkout of the working directory.
func (a *app) checkout() (gitinfo.Info, error) {
	wd, err := os.Getwd()
	if err != nil {
		return gitinfo.Info{}, err
	}
	g, ok := gitinfo.Read(a.ctx, wd)
	if !ok {
		return g, errors.New("not inside a git checkout; run uji from the project you want to build")
	}
	return g, nil
}

// project returns the checkout, its project and the selected app.
func (a *app) project(appKey string) (gitinfo.Info, *config.Project, *config.App, error) {
	g, err := a.checkout()
	if err != nil {
		return g, nil, nil, err
	}
	p, errs := config.Find(g)
	for _, e := range errs {
		ui.Warn(os.Stderr, "skipping a broken project file: %v", e)
	}
	if p == nil {
		return g, nil, nil, fmt.Errorf("no project file matches %s and ujibox found no Gradle app module or Xcode project; run `uji config project`", paths.Pretty(g.Root))
	}
	ap, err := p.App(appKey)
	if err != nil {
		return g, p, nil, err
	}
	return g, p, ap, nil
}

// Execute runs the CLI and returns the exit code.
func Execute() int {
	builder.Version = Version
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := newRoot(ctx)
	if err := root.ExecuteContext(ctx); err != nil {
		if ctx.Err() != nil {
			ui.Fail(os.Stderr, "interrupted")
			return 130
		}
		var quiet *quietError
		if !errors.As(err, &quiet) {
			ui.Fail(os.Stderr, "%v", err)
		}
		return 1
	}
	return 0
}

// quietError is a failure the command already printed.
type quietError struct{ msg string }

func (e *quietError) Error() string { return e.msg }

func newRoot(ctx context.Context) *cobra.Command {
	var a *app
	get := func() *app {
		if a == nil {
			a = newApp(ctx)
		}
		return a
	}
	root := &cobra.Command{
		Use:   "uji",
		Short: "Build, store and install Android and iOS apps on emulators and simulators",
		Long: `uji builds your app, keeps each build outside the project for 30 days,
and installs it on Android emulators, Android devices and iOS simulators.

A device selector is "android", "ios", or a device name: an alias, an AVD
name, a simulator name (add @<os> when it repeats), a serial or a UDID.
With no selector, a command acts on every running device and booted simulator.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !ui.IsTTY(os.Stdout) || !ui.IsTTY(os.Stdin) {
				return cmd.Help()
			}
			return get().openTUI()
		},
	}
	root.CompletionOptions.HiddenDefaultCmd = false
	root.AddGroup(
		&cobra.Group{ID: "build", Title: "Build and install:"},
		&cobra.Group{ID: "device", Title: "Devices:"},
		&cobra.Group{ID: "store", Title: "Stored builds and config:"},
	)
	root.AddCommand(
		cmdBuild(get), cmdInstall(get), cmdSetup(get),
		cmdDevices(get), cmdDeeplink(get), cmdRecord(get), cmdScreenshot(get),
		cmdLaunch(get), cmdUninstall(get), cmdBoot(get), cmdShutdown(get),
		cmdBuilds(get), cmdSync(get), cmdConfig(get), cmdDoctor(get),
	)
	return root
}

func (a *app) openTUI() error {
	// Not a.ctx: Ctrl-C inside a command the screen runs also reaches uji, and
	// that must not cancel the screen.
	d := tui.Deps{Ctx: context.Background(), Mgr: a.mgr, Store: a.store, Title: ui.Dim("all apps")}
	if g, p, ap, err := a.project(""); err == nil {
		d.Filter = store.Filter{AppID: p.StoreID(ap)}
		d.Group = g.Group()
		d.Project = true
		d.Title = ap.Name() + ui.Dim(" · "+g.Group())
	}
	return tui.Run(d)
}
