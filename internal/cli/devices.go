package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/ui"
)

// platformFlags adds --android and --ios to a command.
type platformFlags struct{ android, ios bool }

func (p *platformFlags) add(c *cobra.Command) {
	c.Flags().BoolVarP(&p.android, "android", "A", false, "only Android devices")
	c.Flags().BoolVarP(&p.ios, "ios", "I", false, "only iOS simulators")
}

func cmdDevices(get func() *app) *cobra.Command {
	var all, asJSON bool
	var pf platformFlags
	c := &cobra.Command{
		Use:     "devices [android|ios]",
		Short:   "List Android devices, emulators and iOS simulators",
		GroupID: "device",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			sel := device.ParseSelectors(args, pf.android, pf.ios)
			if len(sel.Names) > 0 {
				return fmt.Errorf("unknown platform %q; use android or ios", sel.Names[0])
			}
			if _, host := sel.Split(); len(host) > 0 {
				return device.NotDeviceError(host[0])
			}
			devs, errs := a.mgr.List(a.ctx, sel.PlatformsOr(device.AllPlatforms))
			if !all {
				var shown []device.Device
				for _, d := range devs {
					if d.State != device.StateOff {
						shown = append(shown, d)
					}
				}
				devs = shown
			}
			if asJSON {
				if devs == nil {
					devs = []device.Device{}
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(devs)
			}
			for _, e := range errs {
				ui.Warn(os.Stderr, "%v", e)
			}
			if len(devs) == 0 {
				fmt.Println("No running devices. `uji devices --all` also lists AVDs and simulators that are off.")
				return nil
			}
			rows := make([][]string, 0, len(devs))
			for _, d := range devs {
				kind := d.Kind
				if d.Wireless {
					// Commands use it only when it is named; see Device.Default.
					kind += " (network)"
				}
				rows = append(rows, []string{d.Platform, deviceName(d), stateText(d.State), kind, d.OS, ui.Dim(d.ID)})
			}
			ui.Table(os.Stdout, []string{"PLATFORM", "NAME", "STATE", "KIND", "OS", "ID"}, rows)
			return nil
		},
	}
	c.Flags().BoolVarP(&all, "all", "a", false, "also list AVDs and simulators that are off")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	pf.add(c)
	return c
}

func deviceName(d device.Device) string {
	name := d.Name
	if d.Alias != "" {
		name = d.Alias + ui.Dim(" ("+d.Name+")")
	}
	return name
}

func stateText(s string) string {
	switch s {
	case device.StateReady:
		return ui.Green(s)
	case device.StateOff:
		return ui.Dim(s)
	default:
		return ui.Yellow(s)
	}
}

func cmdBoot(get func() *app) *cobra.Command {
	return &cobra.Command{
		Use:     "boot <device>...",
		Short:   "Start AVDs or simulators and wait until they are ready",
		GroupID: "device",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			devs, err := a.mgr.Resolve(a.ctx, device.ParseSelectors(args, false, false), device.AllPlatforms)
			if err != nil {
				return err
			}
			return forEach(devs, func(d device.Device) (string, error) {
				if d.Ready() {
					return "already running", nil
				}
				_, err := a.mgr.Boot(a.ctx, d)
				return "booted", err
			})
		},
	}
}

func cmdShutdown(get func() *app) *cobra.Command {
	var pf platformFlags
	c := &cobra.Command{
		Use:     "shutdown [device...]",
		Short:   "Stop emulators or simulators",
		GroupID: "device",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			sel := device.ParseSelectors(args, pf.android, pf.ios)
			if len(sel.Names) == 0 && len(sel.Platforms) == 0 {
				return fmt.Errorf("name the devices to stop, or pass android, ios or all")
			}
			devs, err := a.mgr.Resolve(a.ctx, sel, device.AllPlatforms)
			if err != nil {
				return err
			}
			var running []device.Device
			for _, d := range devs {
				if d.Ready() && d.Kind != device.KindPhysical {
					running = append(running, d)
				}
			}
			return forEach(running, func(d device.Device) (string, error) {
				return "stopped", a.mgr.Shutdown(a.ctx, d)
			})
		},
	}
	pf.add(c)
	return c
}

// result is the outcome of one action on one device.
type result struct {
	dev  device.Device
	note string
	err  error
}

// forEach runs fn on every device in parallel, prints one row per device and
// fails when any device failed.
func forEach(devs []device.Device, fn func(device.Device) (string, error)) error {
	results := make([]result, len(devs))
	var wg sync.WaitGroup
	for i, d := range devs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			note, err := fn(d)
			results[i] = result{dev: d, note: note, err: err}
		}()
	}
	wg.Wait()
	return printResults(results)
}

func printResults(results []result) error {
	failed := 0
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		mark, text := ui.Green("✔"), r.note
		if r.err != nil {
			failed++
			mark, text = ui.Red("✘"), firstLine(r.err.Error())
		}
		rows = append(rows, []string{mark, r.dev.Label(), ui.Dim(r.dev.Platform), text})
	}
	ui.Table(os.Stdout, nil, rows)
	for _, r := range results {
		if r.err != nil && strings.Contains(r.err.Error(), "\n") {
			fmt.Fprintf(os.Stderr, "\n%s:\n%s\n", r.dev.Label(), r.err)
		}
	}
	if failed > 0 {
		return &quietError{fmt.Sprintf("%d of %d devices failed", failed, len(results))}
	}
	return nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
