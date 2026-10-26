# Commands

Run `uji help <command>` for the full help of a command.

A `[selector]` is `android`, `ios` or a device name. See
[Select devices](devices.md). `build`, `setup`, `launch`, `screenshot` and
`record` also take `desktop` and `web`. See
[Desktop and web apps](desktop-web.md).

Every command accepts `--no-sync`. It turns off [auto-sync](sync.md#auto-sync)
for that command.

## Build and install

### `uji build [selector...]`

Builds the app and keeps the artifact in the store. It does not install it.

- With no selector, uji builds every platform of the app.
- `android` or `ios` builds one platform. An iOS build is for simulators.
- `desktop` or `web` builds the [desktop or web app](desktop-web.md).
- A device name builds what that device needs. A physical iPhone needs a
  [device build](projects.md#device-builds).

| Flag | Meaning |
|---|---|
| `--app <name>` | The app to build, when the project has more than one |
| `-C, --clean` | Run the clean steps of the project first |
| `-V, --verbose` | Print the full output of the build tool |
| `-A, --android`, `-I, --ios` | Same as the platform words |
| `--ios-device` | Make the signed iOS build for physical devices |

```sh
uji build
uji build ios --clean
uji build desktop
uji build --ios-device
```

### `uji install [selector...]`

Builds the app, keeps the build in the store, then installs it on each
selected device in parallel and launches it.

- uji makes each kind of build one time, then installs it on every device
  that takes it. Android devices share one APK. iOS simulators share one
  simulator build, and physical iPhones share one device build.
- If you name an AVD or a simulator that is off, uji boots it first.
- With no selector, uji uses every running device. It does not boot a device.
- At the end, uji prints one line for each device. If one device fails, the
  command fails, but the other devices still get the build.

| Flag | Meaning |
|---|---|
| `-L, --last` | Install the newest stored build of this branch. Do not build |
| `-b, --build <ref>` | Install a stored build. See [refs](store.md#refs) |
| `--any-branch` | With `--last`, take the newest build of any branch |
| `--no-launch` | Do not launch the app after the install |
| `--app`, `-C`, `-V`, `-A`, `-I` | Same as `uji build` |

```sh
uji install
uji install android
uji install "iPhone 17 Pro" fold
uji install -L
uji install -b feature/login
```

If the stored build is from another commit than `HEAD`, or from uncommitted
changes, uji shows a warning and installs it.

uji does not install desktop and web builds. Use `uji launch desktop` or
`uji launch web`.

### `uji setup [android|ios|desktop|web]`

Runs the setup steps of the project file without a build. Use it after you
make a new worktree. `uji build` also runs these steps.

## Devices

### `uji devices [android|ios]`

Lists the running Android devices, emulators and booted simulators.

| Flag | Meaning |
|---|---|
| `-a, --all` | Also list the AVDs and simulators that are off |
| `--json` | Print JSON |

### `uji deeplink <url> [selector...]`

Opens a URL on every running device, or on the selected devices.

```sh
uji deeplink "shop://orders/42?tab=details&ref=mail"
uji deeplink "shop://cart" -A    # or --android; -I for iOS
```

!!! warning
    Put the URL in quotes. If you do not, your shell reads `&` and `?` and
    changes the URL before uji gets it.

uji quotes the URL for the Android device shell, so `&`, `#`, spaces and `'`
arrive unchanged.

A physical iPhone cannot open a URL by itself, so uji starts an app with the
URL:

- A web link (`http` or `https`) opens in Safari.
- Another scheme opens in the iOS app of the current project.
- `--bundle <bundle id>` names the app.

### `uji record [path]`

Records the screen of every running device, or of the devices you give with
`-d`. Press ++ctrl+c++ to stop. uji then waits until each recorder finishes
its file.

!!! warning
    Press ++ctrl+c++ one time only. A second press stops the recorders before
    they finish, and the files may not play.

The file name follows the path:

| Path | File |
|---|---|
| none | `./<branch>/<device>-<time>.mp4` |
| a directory, or a path that ends in `/` | `<dir>/<branch>/<device>-<time>.mp4` |
| `demo.mp4` | `demo.mp4` |
| `demo` | `demo.mp4` |

uji creates the branch folder if it does not exist. A `/` in the branch name
becomes `-`, so `feature/login` gives the folder `feature-login`.

If you record more than one device, each file name starts with the device
name. If the working directory is not a git checkout, there is no branch
folder, and the file is `<dir>/<device>-<time>.mp4`.

| Flag | Meaning |
|---|---|
| `-d, --device <name>` | A device to record. Repeat it for more devices |
| `-f, --force` | Overwrite an existing file |
| `--no-window` | Android: record without the scrcpy window |

Android recording uses scrcpy. iOS recording uses `simctl io recordVideo`.

`uji record desktop` and `uji record web` record the window of the app that
`uji launch` started, with `screencapture`. See
[Desktop and web apps](desktop-web.md#screenshot-and-record).

### `uji screenshot [path]`

Saves a PNG of each device screen. The path rules are the same as for
`uji record`, with `.png`. `uji screenshot desktop` and `uji screenshot web`
capture the window of the app that `uji launch` started.

### `uji launch [selector...]` and `uji uninstall [selector...]`

Launch or uninstall the app of the project. uji takes the application id or
bundle id from the newest stored build. If there is no stored build, uji
takes it from the `id` field of the project file.

`uji launch desktop` starts the newest stored desktop build on this Mac.
`uji launch web` serves the newest stored web build and opens it in Chrome,
until you press ++ctrl+c++. See [Desktop and web apps](desktop-web.md#launch).

| Flag | Meaning |
|---|---|
| `--app <name>` | The app, when the project has more than one |
| `-b, --build <ref>` | Desktop and web: launch another stored build |
| `--any-branch` | Desktop and web: take the newest build of any branch |

### `uji boot <device>...` and `uji shutdown <selector>...`

Start AVDs or simulators and wait until they are ready, or stop them.

## Stored builds and config

### `uji builds`

Shows the stored builds of the current project as a tree: app, then branch,
then builds, newest first.

| Subcommand | Action |
|---|---|
| `uji builds ls [--all] [--json]` | List the builds. `--all` lists every app |
| `uji builds pin <ref>` | Keep a build after the retention period |
| `uji builds unpin <ref>` | Let a pinned build expire again |
| `uji builds rm <ref>` | Remove a build |
| `uji builds path <ref>` | Print the path of the APK, the `.app`, the jar or the web folder |
| `uji builds prune [--dry-run]` | Remove the expired builds now |

### `uji sync [selector...]`

Copies the stored builds to the ujibox app on each device. See
[Sync and the ujibox app](sync.md).

uji also syncs after each change to the store, unless you turn off
[auto-sync](sync.md#auto-sync).

### `uji config`

| Subcommand | Action |
|---|---|
| `uji config project [--repo] [--force] [--print]` | Detect the project and write its project file. `--repo` writes `.ujibox.toml` in the checkout, to [share it](projects.md#share-the-config-in-the-repository) |
| `uji config edit [global\|repo]` | Open the project file, `.ujibox.toml`, or the global config, in `$EDITOR` |
| `uji config show` | Print the project config with the variables filled in, and the files it comes from |
| `uji config path [global\|repo]` | Print the path of the file |
| `uji config alias <name> <device>` | Give a device a short name |
| `uji config auto-sync [on\|off]` | Show, or turn on or off, [auto-sync](sync.md#auto-sync) |

### `uji add skills`

Downloads the two agent skills of ujibox and adds them for Claude Code,
Codex and Gemini CLI. uji asks if they go in this repository or in your home
folder; `--local` and `--global` answer ahead. `uji add skill` does the same.
See [Agent skills](skills.md).

```sh
uji add skills
uji add skills --global --claude
```

### `uji doctor`

Checks the tools, the config files and the store. It also checks for Google
Chrome, and for the Screen Recording permission of the terminal, which
desktop and web captures need.

### `uji help [command]` and `uji completion <shell>`

`uji help` prints the help of a command. `uji completion zsh` prints a
completion script. To load it in every zsh session, add this line to
`~/.zshrc`:

```sh
source <(uji completion zsh)
```
