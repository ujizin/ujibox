package cli

import (
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
			builds := map[string][]*store.Manifest{}
			for _, p := range devicePlatforms(devs) {
				builds[p] = a.store.List(store.Filter{AppID: appID, Platform: p})
			}
			for _, p := range devicePlatforms(devs) {
				for _, m := range builds[p] {
					if l := a.store.Use(m); l != nil {
						defer l.Unlock()
					}
				}
			}
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
	c.Flags().StringVarP(&appID, "app", "A", "", "sync only this store app id (see `uji builds --all`)")
	return c
}
