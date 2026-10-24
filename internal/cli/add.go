package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/ujizin/ujibox/internal/paths"
	"github.com/ujizin/ujibox/internal/skills"
	"github.com/ujizin/ujibox/internal/tui"
	"github.com/ujizin/ujibox/internal/ui"
)

func cmdAdd(get func() *app) *cobra.Command {
	c := &cobra.Command{
		Use:     "add",
		Short:   "Add extras to a repository or to this machine",
		GroupID: "store",
	}
	c.AddCommand(cmdAddSkill(get))
	return c
}

func cmdAddSkill(get func() *app) *cobra.Command {
	var claude, codex, gemini, global, local, force bool
	var ref, from string
	c := &cobra.Command{
		Use:     "skill",
		Aliases: []string{"skills"},
		Short:   "Download the agent skills of uji for Claude Code, Codex and Gemini CLI",
		Long: `Download the agent skills of the ujibox repository and add them to the
repository or to your home folder. In a terminal, uji asks where; --local
and --global answer it ahead. The skills teach a coding agent to use uji:

  uji         build, install and check the app on devices
  uji-config  write or fix the project config (.ujibox.toml)

| Agent       | In the repository  | With --global        |
|-------------|--------------------|----------------------|
| Claude Code | .claude/skills/    | ~/.claude/skills/    |
| Codex       | .agents/skills/    | ~/.agents/skills/    |
| Gemini CLI  | .gemini/skills/    | ~/.gemini/skills/    |

With no agent flag, uji adds the skills for each agent that this machine
has, or for all three when it finds none. Gemini CLI also reads
.agents/skills, so with Codex it gets no second copy.`,
		Example: `  uji add skills
  uji add skills --local --claude
  uji add skills --global --force`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := get()
			var src skills.Source = skills.GitHub{Ref: skillRef(ref)}
			if from != "" {
				src = skills.Dir{Path: paths.Expand(from)}
			}
			ix, err := skills.LoadIndex(a.ctx, src)
			if err != nil {
				return err
			}

			if global && local {
				return errors.New("pass --local or --global, not both")
			}
			root := ""
			if g, err := a.checkout(); err == nil {
				root = g.Root
			}
			if !global && !local {
				if !ui.IsTTY(os.Stdin) || !ui.IsTTY(os.Stdout) {
					// A script gets the old default: the repository.
					local = true
				} else {
					global, err = askGlobal(root)
					if err != nil {
						return err
					}
				}
			}
			base := root
			if global {
				base, err = os.UserHomeDir()
				if err != nil {
					return err
				}
			} else if root == "" {
				return errors.New("not inside a git checkout; run uji from the repository, or pass --global")
			}

			failed := false
			for _, ag := range pickAgents(claude, codex, gemini) {
				dir := filepath.Join(base, filepath.FromSlash(ag.Dir))
				for _, s := range ix.Skills {
					dest, err := skills.Install(a.ctx, src, s, dir, force)
					if err != nil {
						ui.Fail(os.Stderr, "%s %s: %v", ag.Name, s.Name, err)
						failed = true
						continue
					}
					ui.OK(os.Stdout, "%-7s %s → %s", ag.Name, s.Name, paths.Pretty(dest))
				}
			}
			if failed {
				return &quietError{"some skills were not added"}
			}
			if !global {
				fmt.Println(ui.Dim("Commit the skill folders to share them with everyone who clones the repository."))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&claude, "claude", false, "add the skills for Claude Code")
	c.Flags().BoolVar(&codex, "codex", false, "add the skills for Codex")
	c.Flags().BoolVar(&gemini, "gemini", false, "add the skills for Gemini CLI")
	c.Flags().BoolVarP(&local, "local", "l", false, "add the skills to this repository, without asking")
	c.Flags().BoolVarP(&global, "global", "g", false, "add the skills to your home folder, for every repository, without asking")
	c.Flags().BoolVarP(&force, "force", "f", false, "replace the skills that are there already, to update them")
	c.Flags().StringVar(&ref, "ref", "", "the git ref of the ujibox repository to download from (default: this uji's version, or main)")
	c.Flags().StringVar(&from, "from", "", "read the skills from a local folder instead of GitHub")
	return c
}

// askGlobal asks where to add the skills, with a list that the arrow keys
// move. It returns true for the home folder. Outside a checkout, the home
// folder is the only place to offer.
func askGlobal(root string) (bool, error) {
	options := []tui.Option{
		{Label: "this repository", Note: paths.Pretty(root)},
		{Label: "your home folder", Note: "for every repository"},
	}
	start := 0
	if root == "" {
		options = []tui.Option{
			{Label: "your home folder", Note: "for every repository"},
			{Label: "cancel", Note: "this folder is not a git checkout"},
		}
	}
	i, err := tui.Choose("Where should uji add the skills?", options, start)
	switch {
	case errors.Is(err, tui.ErrCanceled):
		return false, &quietError{"canceled"}
	case err != nil:
		return false, err
	}
	if root == "" {
		if i == 1 {
			return false, &quietError{"canceled"}
		}
		return true, nil
	}
	return i == 1, nil
}

var releaseRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// skillRef is the git ref to download skills from: the release tag of this
// uji, so the skills match the commands it has, or main for a dev build.
func skillRef(flag string) string {
	if flag != "" {
		return flag
	}
	if releaseRe.MatchString(Version) {
		return Version
	}
	return "main"
}

// pickAgents returns the agents of the flags. With no flag, it returns the
// agents found on this machine, or all of them when it finds none. Gemini CLI
// reads .agents/skills too, so with Codex it needs no copy of its own.
func pickAgents(claude, codex, gemini bool) []skills.Agent {
	want := map[string]bool{"claude": claude, "codex": codex, "gemini": gemini}
	if !claude && !codex && !gemini {
		home, _ := os.UserHomeDir()
		for _, ag := range skills.Agents {
			_, err := exec.LookPath(ag.Tool)
			_, serr := os.Stat(filepath.Join(home, ag.Home))
			want[ag.Name] = err == nil || serr == nil
		}
		if !want["claude"] && !want["codex"] && !want["gemini"] {
			want = map[string]bool{"claude": true, "codex": true, "gemini": true}
		}
	}
	var out []skills.Agent
	for _, ag := range skills.Agents {
		if !want[ag.Name] {
			continue
		}
		if ag.Name == "gemini" && want["codex"] {
			ui.Info(os.Stderr, "gemini: reads the codex copy in .agents/skills")
			continue
		}
		out = append(out, ag)
	}
	return out
}
