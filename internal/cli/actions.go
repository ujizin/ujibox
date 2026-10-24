package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/gitinfo"
	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdDeeplink(get func() *app) *cobra.Command {
	var pf platformFlags
	var bundle string
	c := &cobra.Command{
		Use:   "deeplink <url> [android|ios|device...]",
		Short: "Open a deeplink on devices",
		Long: `Open a URL on every running device, or on the selected ones. Quote the URL
in your shell: an unquoted & or ? is read by the shell, not by uji.

A physical iPhone cannot open a URL by itself; uji starts an app with it. A
web link opens in Safari. Another scheme opens in the iOS app of the current
project. --bundle names the app.`,
		Example: `  uji deeplink "myapp://orders/42?tab=details"
  uji deeplink "myapp://home" --android
  uji deeplink "https://example.com/promo" "iPhone 17 Pro"`,
		GroupID: "device",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			url := args[0]
			if !strings.Contains(url, ":") {
				return fmt.Errorf("%q has no scheme; write it as scheme://path", url)
			}
			devs, err := a.mgr.Resolve(a.ctx, device.ParseSelectors(args[1:], pf.android, pf.ios), device.AllPlatforms)
			if err != nil {
				return err
			}
			devs, err = a.mgr.EnsureReady(a.ctx, devs, func(s string) { ui.Info(os.Stderr, "%s", s) })
			if err != nil {
				return err
			}
			app := bundle
			if app == "" && !isWebURL(url) && hasPhysicalIOS(devs) {
				app = a.projectBundleID()
			}
			return forEach(devs, func(d device.Device) (string, error) {
				return "opened", a.mgr.OpenURL(a.ctx, d, url, app)
			})
		},
	}
	pf.add(c)
	c.Flags().StringVar(&bundle, "bundle", "", "physical iPhone: the bundle id of the app that opens the URL")
	return c
}

func isWebURL(url string) bool {
	u := strings.ToLower(url)
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}

func hasPhysicalIOS(devs []device.Device) bool {
	for _, d := range devs {
		if d.Target() == device.TargetDevice {
			return true
		}
	}
	return false
}

// projectBundleID returns the iOS bundle id of the current project's app:
// from the project file, or from its newest stored iOS build. It returns ""
// outside a project.
func (a *app) projectBundleID() string {
	_, p, ap, err := a.project("")
	if err != nil || ap.IOS == nil {
		return ""
	}
	if ap.IOS.ID != "" {
		return ap.IOS.ID
	}
	for _, m := range a.store.List(store.Filter{AppID: p.StoreID(ap), Platform: config.IOS}) {
		if m.PackageID != "" {
			return m.PackageID
		}
	}
	return ""
}

// mediaFlags select devices for record and screenshot, whose positional
// argument is the output path.
type mediaFlags struct {
	devices []string
	force   bool
	platformFlags
}

func (m *mediaFlags) add(c *cobra.Command) {
	c.Flags().StringArrayVarP(&m.devices, "device", "d", nil, "device to use (repeatable); default: every running device")
	c.Flags().BoolVarP(&m.force, "force", "f", false, "overwrite an existing file")
	m.platformFlags.add(c)
}

func (a *app) mediaTargets(mf mediaFlags) ([]device.Device, error) {
	devs, err := a.mgr.Resolve(a.ctx, device.ParseSelectors(mf.devices, mf.android, mf.ios), device.AllPlatforms)
	if err != nil {
		return nil, err
	}
	return a.mgr.EnsureReady(a.ctx, devs, func(s string) { ui.Info(os.Stderr, "%s", s) })
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._@-]+`)

// outputPaths decides the file of each device.
//   - no path, an existing directory, or a path that ends in /: a directory;
//     uji writes <dir>/<branch>/<timestamp><ext>, or <dir>/<device>-<timestamp><ext>
//     outside a git checkout
//   - a path that ends in ext: that file
//   - any other path: that file plus ext
//
// With several devices, each name gets the device name.
func (a *app) outputPaths(arg, ext string, devs []device.Device, force bool) ([]string, error) {
	stamp := time.Now().Format("20060102-150405")
	branch := ""
	if wd, err := os.Getwd(); err == nil {
		if g, ok := gitinfo.Read(a.ctx, wd); ok {
			branch = g.Group()
		}
	}
	clean := func(s string) string { return strings.Trim(unsafeName.ReplaceAllString(s, "-"), "-") }

	arg = paths.Expand(arg)
	isDir := arg == "" || strings.HasSuffix(arg, "/")
	if st, err := os.Stat(arg); err == nil && st.IsDir() {
		isDir = true
	}
	out := make([]string, len(devs))
	for i, d := range devs {
		var p string
		if isDir {
			dir := arg
			if branch != "" {
				dir = filepath.Join(arg, clean(branch))
			}
			parts := []string{}
			if branch == "" || len(devs) > 1 {
				parts = append(parts, clean(d.Label()))
			}
			parts = append(parts, stamp)
			p = filepath.Join(dir, strings.Join(parts, "-")+ext)
		} else {
			p = strings.TrimSuffix(arg, ext)
			if len(devs) > 1 {
				p += "-" + clean(d.Label())
			}
			p += ext
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(abs); err == nil && !force {
			return nil, fmt.Errorf("%s exists; pass --force to overwrite it", paths.Pretty(abs))
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		out[i] = abs
	}
	return out, nil
}

func cmdScreenshot(get func() *app) *cobra.Command {
	var mf mediaFlags
	c := &cobra.Command{
		Use:   "screenshot [path]",
		Short: "Save a PNG of each device screen",
		Long: `Save a PNG of the screen of every running device, or of the devices given
with --device. With a directory path (or no path), uji writes the file into a
folder named after the branch: <dir>/<branch>/<timestamp>.png.`,
		GroupID: "device",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			devs, err := a.mediaTargets(mf)
			if err != nil {
				return err
			}
			files, err := a.outputPaths(firstArg(args), ".png", devs, mf.force)
			if err != nil {
				return err
			}
			idx := map[string]int{}
			for i, d := range devs {
				idx[d.Platform+d.ID] = i
			}
			return forEach(devs, func(d device.Device) (string, error) {
				f := files[idx[d.Platform+d.ID]]
				return paths.Pretty(f), a.mgr.Screenshot(a.ctx, d, f)
			})
		},
	}
	mf.add(c)
	return c
}

func cmdRecord(get func() *app) *cobra.Command {
	var mf mediaFlags
	var noWindow bool
	c := &cobra.Command{
		Use:   "record [path]",
		Short: "Record the screen of devices to mp4 until Ctrl-C",
		Long: `Record the screen of every running device, or of the devices given with
--device, until you press Ctrl-C. uji then waits for each recorder to finish
its file.

With a directory path (or no path), uji writes the file into a folder named
after the branch: <dir>/<branch>/<timestamp>.mp4.
Android uses scrcpy; iOS uses simctl recordVideo.`,
		Example: `  uji record
  uji record ~/Movies
  uji record demo.mp4 -d fold`,
		GroupID: "device",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			devs, err := a.mediaTargets(mf)
			if err != nil {
				return err
			}
			files, err := a.outputPaths(firstArg(args), ".mp4", devs, mf.force)
			if err != nil {
				return err
			}
			return a.record(devs, files, noWindow)
		},
	}
	mf.add(c)
	c.Flags().BoolVar(&noWindow, "no-window", false, "Android: record without the scrcpy mirror window")
	return c
}

func (a *app) record(devs []device.Device, files []string, noWindow bool) error {
	// Take Ctrl-C here, so uji lives to wait for the recorders. Each recorder
	// runs in its own process group and gets exactly one SIGINT from uji: a
	// second SIGINT makes scrcpy quit before it writes the mp4 index.
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	type rec struct {
		dev    device.Device
		file   string
		cmd    *exec.Cmd
		stderr *strings.Builder
		err    error
	}
	var recs []*rec
	for i, d := range devs {
		cmd, err := a.mgr.RecordCmd(d, files[i], noWindow)
		if err != nil {
			for _, r := range recs {
				_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGINT)
			}
			return err
		}
		r := &rec{dev: d, file: files[i], cmd: cmd, stderr: &strings.Builder{}}
		cmd.Stdout = r.stderr
		cmd.Stderr = r.stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start recording on %s: %w", d.Label(), err)
		}
		recs = append(recs, r)
		fmt.Fprintf(os.Stderr, "%s %s → %s\n", ui.Red("●"), d.Label(), paths.Pretty(files[i]))
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	for _, r := range recs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.err = r.cmd.Wait()
		}()
	}
	go func() { wg.Wait(); close(done) }()

	fmt.Fprintln(os.Stderr, ui.Dim("Recording. Press Ctrl-C to stop."))
	stopped := false
loop:
	for {
		select {
		case <-done:
			break loop
		case <-sigs:
			if stopped {
				ui.Warn(os.Stderr, "still writing the files; wait, or press Ctrl-C again to abort them")
				select {
				case <-done:
					break loop
				case <-sigs:
					for _, r := range recs {
						_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL)
					}
					<-done
					return errors.New("recording aborted; the files may not play")
				}
			}
			stopped = true
			fmt.Fprintln(os.Stderr)
			ui.Info(os.Stderr, "stopping; finishing the files…")
			for _, r := range recs {
				// A recorder that already exited gives ESRCH, which is harmless.
				_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGINT)
			}
		}
	}

	results := make([]result, len(recs))
	for i, r := range recs {
		results[i] = result{dev: r.dev}
		st, err := os.Stat(r.file)
		switch {
		case err != nil || st.Size() == 0:
			msg := strings.TrimSpace(r.stderr.String())
			if msg == "" && r.err != nil {
				msg = r.err.Error()
			}
			results[i].err = fmt.Errorf("no video written: %s", lastNonEmpty(msg))
		default:
			results[i].note = fmt.Sprintf("%s (%s)", paths.Pretty(r.file), ui.Size(st.Size()))
		}
	}
	return printResults(results)
}

func lastNonEmpty(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// appID returns the application id or bundle id that launch and uninstall use.
func (a *app) appID(appKey, platform string) (string, error) {
	_, p, ap, err := a.project(appKey)
	if err != nil {
		return "", err
	}
	pl := ap.Platform(platform)
	if pl == nil {
		return "", fmt.Errorf("%s has no %s config", ap.Name(), platform)
	}
	// The stored build has the real id, with flavor and build-type suffixes.
	if list := a.store.List(store.Filter{AppID: p.StoreID(ap), Platform: platform}); len(list) > 0 && list[0].PackageID != "" {
		return list[0].PackageID, nil
	}
	if pl.ID == "" {
		return "", fmt.Errorf("no id for %s %s: build it once, or set apps.%s.%s.id in %s", ap.Name(), platform, ap.Key, platform, projectFileText(p))
	}
	return pl.ID, nil
}

func projectFileText(p *config.Project) string {
	if p.EditFile() == "" {
		return "a project file (`uji config project`)"
	}
	return paths.Pretty(p.EditFile())
}

func appCommand(get func() *app, use, short, verb string, fn func(a *app, d device.Device, id string) error) *cobra.Command {
	var pf platformFlags
	var appKey string
	c := &cobra.Command{
		Use:     use + " [android|ios|device...]",
		Short:   short,
		GroupID: "device",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			devs, err := a.mgr.Resolve(a.ctx, device.ParseSelectors(args, pf.android, pf.ios), device.AllPlatforms)
			if err != nil {
				return err
			}
			ids := map[string]string{}
			idErrs := map[string]error{}
			for _, p := range devicePlatforms(devs) {
				ids[p], idErrs[p] = a.appID(appKey, p)
			}
			devs, err = a.mgr.EnsureReady(a.ctx, devs, func(s string) { ui.Info(os.Stderr, "%s", s) })
			if err != nil {
				return err
			}
			return forEach(devs, func(d device.Device) (string, error) {
				if e := idErrs[d.Platform]; e != nil {
					return "", e
				}
				return verb + " " + ids[d.Platform], fn(a, d, ids[d.Platform])
			})
		},
	}
	pf.add(c)
	c.Flags().StringVar(&appKey, "app", "", "app of the project")
	return c
}

func cmdLaunch(get func() *app) *cobra.Command {
	return appCommand(get, "launch", "Launch the app on devices", "launched", func(a *app, d device.Device, id string) error {
		return a.mgr.Launch(a.ctx, d, id)
	})
}

func cmdUninstall(get func() *app) *cobra.Command {
	return appCommand(get, "uninstall", "Uninstall the app from devices", "removed", func(a *app, d device.Device, id string) error {
		return a.mgr.Uninstall(a.ctx, d, id)
	})
}
