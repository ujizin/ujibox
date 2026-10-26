package cli

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/proc"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdLaunch(get func() *app) *cobra.Command {
	var pf platformFlags
	var appKey, ref string
	var anyBranch bool
	c := &cobra.Command{
		Use:   "launch [android|ios|desktop|web|device...]",
		Short: "Launch the app on devices, or the desktop or web app on this Mac",
		Long: `Launch the installed app on every running device, or on the selected ones.

"desktop" starts the newest stored desktop build of this branch on this Mac.
"web" serves the newest stored web build on a free localhost port and opens
it in a Chrome window with a profile of its own. uji keeps the server alive
until you press Ctrl-C. --build picks another stored build.

` + "`uji screenshot desktop`" + ` and ` + "`uji record web`" + ` capture the window of the
app that uji launched.`,
		Example: `  uji launch
  uji launch android
  uji launch desktop
  uji launch web -b feature/login`,
		GroupID: "device",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			sel := device.ParseSelectors(args, pf.android, pf.ios)
			devSel, host := sel.Split()
			if len(host) > 0 {
				if len(host) > 1 || len(devSel.Platforms) > 0 || len(devSel.Names) > 0 {
					return errors.New("launch desktop or web on its own: `uji launch desktop` or `uji launch web`")
				}
				return a.launchHost(host[0], appKey, ref, anyBranch)
			}
			if ref != "" {
				return errors.New("--build works with desktop and web; on a device, use `uji install -b " + ref + "`")
			}
			return a.forDevicesWithID(sel, appKey, "launched", func(d device.Device, id string) error {
				return a.mgr.Launch(a.ctx, d, id)
			})
		},
	}
	pf.add(c)
	c.Flags().StringVar(&appKey, "app", "", "app of the project")
	c.Flags().StringVarP(&ref, "build", "b", "", "desktop and web: launch a stored build: N, build id, commit sha or branch")
	c.Flags().BoolVar(&anyBranch, "any-branch", false, "desktop and web: take the newest build of any branch")
	return c
}

// launchHost starts a stored desktop or web build on this Mac.
func (a *app) launchHost(platform, appKey, ref string, anyBranch bool) error {
	g, p, ap, err := a.project(appKey)
	if err != nil {
		return err
	}
	if ap.Platform(platform) == nil {
		return fmt.Errorf("%s has no %s config; add [apps.%s.%s] to %s", ap.Name(), platform, ap.Key, platform, projectFileText(p))
	}
	if ref == "" {
		ref = "last"
	}
	m, err := a.pick(g, p, ap, device.DefaultTarget(platform), ref, anyBranch)
	if err != nil {
		return err
	}
	if l := a.store.Use(m); l != nil {
		defer l.Unlock()
	}
	if platform == config.Desktop {
		return a.launchDesktop(g, p, ap, m)
	}
	return a.serveWeb(p, ap, m)
}

func (a *app) launchDesktop(g gitinfo.Info, p *config.Project, ap *config.App, m *store.Manifest) error {
	// Like a device launch, a new launch replaces the running copy.
	for _, l := range device.ListLaunched([]string{config.Desktop}) {
		if l.AppID == p.StoreID(ap) {
			ui.Info(os.Stderr, "stopping the running %s (pid %d)", l.App, l.PID)
			l.Stop()
		}
	}
	logFile := filepath.Join(paths.Cache(), "logs", fmt.Sprintf("%s-desktop-run-%s.log", ap.Key, time.Now().Format("20060102-150405")))
	env := proc.Env(nil, appEnv(config.NewVars(g, ap, config.Desktop), ap)...)
	pid, command, err := device.LaunchDesktop(a.ctx, m.ArtifactPath(), logFile, env)
	if err != nil {
		return err
	}
	l := device.Launched{Platform: config.Desktop, PID: pid, Command: command, App: ap.Name(), AppID: p.StoreID(ap), Build: m.ShortID(), StartedAt: time.Now()}
	if err := device.SaveLaunched(l); err != nil {
		ui.Warn(os.Stderr, "uji cannot record the launch, so screenshot and record will not find the app: %v", err)
	}
	ui.OK(os.Stderr, "launched %s %s (pid %d)", ap.Name(), versionText(m), pid)
	ui.Info(os.Stderr, "%s", ui.Dim("output: "+paths.Pretty(logFile)))
	return nil
}

// appEnv returns the env of the app and of its desktop platform as pairs,
// the same values that the build gets.
func appEnv(v config.Vars, ap *config.App) []string {
	var out []string
	for k, val := range ap.Env {
		out = append(out, k+"="+v.Expand(val))
	}
	if ap.Desktop != nil {
		for k, val := range ap.Desktop.Env {
			out = append(out, k+"="+v.Expand(val))
		}
	}
	return out
}

// serveWeb serves the dist folder of a web build on localhost and opens it in
// a Chrome window that uji controls. It returns after Ctrl-C.
func (a *app) serveWeb(p *config.Project, ap *config.App, m *store.Manifest) error {
	for _, l := range device.ListLaunched([]string{config.Web}) {
		if l.AppID == p.StoreID(ap) {
			return fmt.Errorf("another uji serves %s at %s (Chrome pid %d); press Ctrl-C there first", l.App, l.URL, l.PID)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on localhost: %w", err)
	}
	url := fmt.Sprintf("http://localhost:%d/", ln.Addr().(*net.TCPAddr).Port)
	// Browsers compile wasm while it downloads only with this type.
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	srv := &http.Server{Handler: noCache(http.FileServer(http.Dir(m.ArtifactPath())))}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	ui.OK(os.Stderr, "serving %s %s at %s", ap.Name(), m.ShortID(), url)

	chrome := device.ChromePath()
	if chrome == "" {
		ui.Warn(os.Stderr, "Google Chrome is not installed; open %s in a browser. screenshot and record need the Chrome window", url)
	} else {
		profile := filepath.Join(paths.Cache(), "chrome", p.StoreID(ap))
		cmd := device.ChromeCmd(chrome, url, profile)
		if err := cmd.Start(); err != nil {
			ui.Warn(os.Stderr, "start Chrome: %v; open %s in a browser", err, url)
		} else {
			l := device.Launched{Platform: config.Web, PID: cmd.Process.Pid, Command: chrome, App: ap.Name(), AppID: p.StoreID(ap),
				Build: m.ShortID(), URL: url, StartedAt: time.Now()}
			if err := device.SaveLaunched(l); err != nil {
				ui.Warn(os.Stderr, "uji cannot record the launch, so screenshot and record will not find the window: %v", err)
			}
			go func() { _ = cmd.Wait() }()
			defer func() {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
				device.RemoveLaunched(l)
			}()
			ui.Info(os.Stderr, "%s", ui.Dim("Chrome profile: "+paths.Pretty(profile)))
		}
	}
	fmt.Fprintln(os.Stderr, ui.Dim("Serving. Press Ctrl-C to stop."))
	select {
	case <-a.ctx.Done():
		fmt.Fprintln(os.Stderr)
		ui.Info(os.Stderr, "stopped serving %s", ap.Name())
		return nil
	case err := <-serveErr:
		return fmt.Errorf("web server: %w", err)
	}
}

// noCache makes the browser load each new build, not a cached older one.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
