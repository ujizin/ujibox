package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdBuilds(get func() *app) *cobra.Command {
	var lf listFlags
	c := &cobra.Command{
		Use:   "builds",
		Short: "List, pin and remove stored builds",
		Long: `Show the builds in the store as a tree: app, then branch, then builds,
newest first. The number before a build selects it in ` + "`uji install -b N`" + `
from that branch.

Builds expire after the retention period (30 days by default). The newest
builds of each branch and every pinned build stay.`,
		GroupID: "store",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listBuilds(get(), lf)
		},
	}
	lf.add(c)

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List stored builds (the default)",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return listBuilds(get(), lf) },
	}
	lf.add(ls)

	pin := func(use, short string, pinned bool) *cobra.Command {
		var sf scopeFlags
		c := &cobra.Command{
			Use:   use + " <ref>",
			Short: short,
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				a := get()
				m, err := a.findBuild(args[0], sf)
				if err != nil {
					return err
				}
				if err := a.store.SetPinned(m, pinned); err != nil {
					return err
				}
				state := "unpinned"
				if pinned {
					state = "pinned"
				}
				ui.OK(os.Stdout, "%s build %s (%s %s %s)", state, m.ShortID(), m.App.Name, m.Platform, m.Group())
				a.autoSync(m.Platform)
				return nil
			},
		}
		sf.add(c)
		return c
	}

	var rmScope scopeFlags
	rm := &cobra.Command{
		Use:   "rm <ref>",
		Short: "Remove a stored build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			m, err := a.findBuild(args[0], rmScope)
			if err != nil {
				return err
			}
			if err := a.store.Remove(m); err != nil {
				return err
			}
			ui.OK(os.Stdout, "removed build %s (%s %s %s)", m.ShortID(), m.App.Name, m.Platform, m.Group())
			a.autoSync(m.Platform)
			return nil
		},
	}
	rmScope.add(rm)

	var pathScope scopeFlags
	path := &cobra.Command{
		Use:   "path <ref>",
		Short: "Print the artifact path of a stored build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			m, err := a.findBuild(args[0], pathScope)
			if err != nil {
				return err
			}
			fmt.Println(m.ArtifactPath())
			return nil
		},
	}
	pathScope.add(path)

	var dryRun bool
	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove expired builds now",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			removed, errs := a.store.Prune(a.policy(), time.Now(), dryRun)
			verb := "removed"
			if dryRun {
				verb = "would remove"
			}
			var freed int64
			for _, m := range removed {
				freed += m.Artifact.Size
				ui.Info(os.Stdout, "%s %s %s %s %s (built %s)", verb, m.App.Name, m.Platform, m.Group(), m.ShortID(), ui.Ago(m.CreatedAt))
			}
			for _, e := range errs {
				ui.Warn(os.Stderr, "%v", e)
			}
			ui.OK(os.Stdout, "%s %s, %s", verb, ui.Plural(len(removed), "build"), ui.Size(freed))
			if !dryRun {
				platforms := map[string]bool{}
				for _, m := range removed {
					platforms[m.Platform] = true
				}
				var list []string
				for p := range platforms {
					list = append(list, p)
				}
				sort.Strings(list)
				a.autoSync(list...)
			}
			return nil
		},
	}
	prune.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "list the builds without removing them")

	c.AddCommand(ls, pin("pin", "Keep a build past the retention period", true),
		pin("unpin", "Let a pinned build expire again", false), rm, path, prune)
	return c
}

type listFlags struct {
	json bool
	all  bool
	scopeFlags
}

func (l *listFlags) add(c *cobra.Command) {
	c.Flags().BoolVar(&l.json, "json", false, "print the manifests as JSON")
	c.Flags().BoolVar(&l.all, "all", false, "list the builds of every app, not only the current project's")
	l.scopeFlags.add(c)
}

// scopeFlags narrow the builds a ref searches.
type scopeFlags struct {
	app      string
	platform string
}

func (s *scopeFlags) add(c *cobra.Command) {
	c.Flags().StringVar(&s.app, "app", "", "app of the project")
	c.Flags().StringVarP(&s.platform, "platform", "p", "", "android or ios")
}

// filter scopes to the app of the current checkout when there is one.
func (a *app) filter(sf scopeFlags, everything bool) (store.Filter, string) {
	f := store.Filter{Platform: sf.platform}
	group := ""
	if everything {
		return f, group
	}
	g, p, ap, err := a.project(sf.app)
	if err == nil {
		f.AppID = p.StoreID(ap)
		group = g.Group()
	}
	return f, group
}

func (a *app) findBuild(ref string, sf scopeFlags) (*store.Manifest, error) {
	f, group := a.filter(sf, false)
	f.Group = group
	return a.store.Find(ref, f)
}

func listBuilds(a *app, lf listFlags) error {
	f, current := a.filter(lf.scopeFlags, lf.all)
	list := a.store.List(f)
	if lf.json {
		if list == nil {
			list = []*store.Manifest{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}
	if len(list) == 0 {
		if f.AppID != "" {
			fmt.Printf("No stored builds for this project. `uji build` makes one; `uji builds --all` lists every app.\n")
		} else {
			fmt.Println("No stored builds. `uji build` makes one.")
		}
		return nil
	}

	type key struct{ app, group string }
	var apps []string
	appNames := map[string]string{}
	groups := map[string][]string{}
	byGroup := map[key][]*store.Manifest{}
	for _, m := range list {
		if _, ok := appNames[m.App.ID]; !ok {
			apps = append(apps, m.App.ID)
			appNames[m.App.ID] = m.App.Name
		}
		k := key{m.App.ID, m.Group()}
		if _, ok := byGroup[k]; !ok {
			groups[m.App.ID] = append(groups[m.App.ID], m.Group())
		}
		byGroup[k] = append(byGroup[k], m)
	}
	var total int64
	for _, appID := range apps {
		fmt.Println(ui.Bold(appNames[appID]) + ui.Dim("  "+appID))
		for _, grp := range groups[appID] {
			label := grp
			if grp == current {
				label += ui.Green("  ← HEAD")
			}
			fmt.Println("  " + label)
			var rows [][]string
			for i, m := range byGroup[key{appID, grp}] {
				total += m.Artifact.Size
				flags := ""
				if m.Pinned {
					flags += ui.Yellow("pinned ")
				}
				if m.Git.Dirty {
					flags += ui.Yellow("dirty")
				}
				rows = append(rows, []string{
					"    " + fmt.Sprint(i+1), m.Platform, versionText(m), m.ShortSHA(), ui.Ago(m.CreatedAt),
					ui.Size(m.Artifact.Size), ui.Dim(m.ShortID()), flags,
				})
			}
			ui.Table(os.Stdout, nil, rows)
		}
	}
	fmt.Println(ui.Dim(fmt.Sprintf("\n%s, %s, in %s", ui.Plural(len(list), "build"), ui.Size(total), paths.Pretty(a.store.Root))))
	return nil
}
