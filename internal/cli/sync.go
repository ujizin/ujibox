package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/syncer"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdSync(get func() *app) *cobra.Command {
	var pf platformFlags
	var appID string
	c := &cobra.Command{
		Use:   "sync [android|ios|device...]",
		Short: "Copy the stored builds into the ujibox app on devices",
		Long: `Copy every stored build of a device's platform into the ujibox app on that
device: APKs to Android devices, .app bundles to iOS simulators. The device
then holds exactly the builds in the store; builds that left the store are
removed from it.

On Android, the ujibox app installs a build itself. iOS does not let an app
install other apps, so on a simulator the ujibox app lists the builds and
copies the command that installs one; run that command in a Mac terminal.`,
		Example: `  uji sync
  uji sync android
  uji sync "Galaxy S24"`,
		GroupID: "store",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			devs, err := a.mgr.Resolve(a.ctx, device.ParseSelectors(args, pf.android, pf.ios), device.AllPlatforms)
			if err != nil {
				return err
			}
			devs, err = a.mgr.EnsureReady(a.ctx, devs, func(s string) { ui.Info(os.Stderr, "%s", s) })
			if err != nil {
				return err
			}
			for _, d := range devs {
				if d.Platform == config.IOS {
					ui.Warn(os.Stderr, "iOS: the ujibox app cannot install builds by itself; it copies the install command for a Mac terminal")
					break
				}
			}
			builds, unlock := a.syncBuilds(devicePlatforms(devs), appID)
			defer unlock()
			return forEach(devs, func(d device.Device) (string, error) {
				r, err := syncer.Sync(a.ctx, a.mgr, a.global, d, builds[d.Platform])
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s (%d copied, %d removed)", ui.Plural(r.Pushed+r.Kept, "build"), r.Pushed, r.Removed), nil
			})
		},
	}
	pf.add(c)
	c.Flags().StringVar(&appID, "app", "", "sync only this store app id (see `uji builds --all`)")
	return c
}

// syncBuilds lists the stored builds of each platform and holds them, so
// prune cannot remove one during the copy. Call unlock after the sync.
func (a *app) syncBuilds(platforms []string, appID string) (map[string][]*store.Manifest, func()) {
	builds := map[string][]*store.Manifest{}
	var locks []interface{ Unlock() }
	for _, p := range platforms {
		builds[p] = a.store.List(store.Filter{AppID: appID, Platform: p})
		for _, m := range builds[p] {
			if l := a.store.Use(m); l != nil {
				locks = append(locks, l)
			}
		}
	}
	return builds, func() {
		for _, l := range locks {
			l.Unlock()
		}
	}
}

// autoSync syncs the ujibox app on each running device of the platforms,
// after a command changed the store. It never fails the command: it skips a
// device without the ujibox app, and it prints other problems as warnings.
func (a *app) autoSync(platforms ...string) {
	if a.noSync || !a.global.AutoSyncOn() || a.ctx.Err() != nil || len(platforms) == 0 {
		return
	}
	devs, _ := a.mgr.List(a.ctx, platforms)
	var ready []device.Device
	for _, d := range devs {
		if d.Ready() {
			ready = append(ready, d)
		}
	}
	if len(ready) == 0 {
		return
	}
	builds, unlock := a.syncBuilds(devicePlatforms(ready), "")
	defer unlock()
	for _, d := range ready {
		r, err := syncer.Sync(a.ctx, a.mgr, a.global, d, builds[d.Platform])
		switch {
		case errors.Is(err, syncer.ErrNoApp):
			continue
		case err != nil:
			ui.Warn(os.Stderr, "auto-sync %s: %v", d.Label(), err)
		case r.Pushed > 0 || r.Removed > 0:
			ui.Info(os.Stderr, "auto-sync %s: %s (%d copied, %d removed)", d.Label(), ui.Plural(r.Pushed+r.Kept, "build"), r.Pushed, r.Removed)
		}
	}
}
