# Agent skills

The ujibox repository has agent skills: short guides that teach a coding
agent to use uji. With them, Claude Code, Codex or Gemini CLI can build the
app, install it, and take a screenshot to check its own change.

| Skill | What it teaches |
|---|---|
| `uji` | Build, install and check an app on emulators, simulators and devices |
| `uji-config` | Write or fix the project config, including `.ujibox.toml` |

## Add the skills

```sh
uji add skills
```

uji asks where to add the skills. Move with the arrow keys, and press Enter
to choose. Esc cancels.

```text
Where should uji add the skills?
❯ this repository  ~/code/shop
  your home folder  for every repository
↑/↓ to move, Enter to choose, Esc to cancel
```

`--local` and `--global` answer the question ahead. Outside a git checkout,
uji offers only your home folder. With no terminal, for example in a script,
uji adds the skills to the repository. `uji add skill` does the same as
`uji add skills`.

uji downloads the skills from GitHub and writes them where each agent reads
them:

| Agent | In the repository | With `--global` |
|---|---|---|
| Claude Code | `.claude/skills/` | `~/.claude/skills/` |
| Codex | `.agents/skills/` | `~/.agents/skills/` |
| Gemini CLI | `.gemini/skills/` | `~/.gemini/skills/` |

- With no agent flag, uji adds the skills for each agent that this machine
  has. If it finds none, it adds them for all three.
- `--claude`, `--codex` and `--gemini` pick the agents.
- Gemini CLI also reads `.agents/skills`. Thus with Codex, Gemini CLI gets no
  second copy.

In a repository, commit the skill folders. Then everyone who clones the
repository gets them.

| Flag | Meaning |
|---|---|
| `--claude`, `--codex`, `--gemini` | The agents to add the skills for |
| `-l, --local` | Add the skills to this repository, without the question |
| `-g, --global` | Add the skills to your home folder, without the question |
| `-f, --force` | Replace the skills that are there already. Use it to update them |
| `--ref <ref>` | The git ref to download from. The default is the release of your uji, or `main` |
| `--from <dir>` | Read the skills from a local folder, for example a clone of ujibox |
