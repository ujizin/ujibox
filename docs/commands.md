# Commands

Run `uji help <command>` for the full help of a command.

A `[selector]` is `android`, `ios` or a device name. See
[Select devices](devices.md).

Every command accepts `--no-sync`. It turns off [auto-sync](sync.md#auto-sync)
for that command.

## Build and install

### `uji build [selector...]`

Builds the app and keeps the artifact in the store. It does not install it.

- With no selector, uji builds every platform of the app.
- `android` or `ios` builds one platform.
- A device name builds the platform of that device.

| Flag | Meaning |
|---|---|
| `-A, --app <name>` | The app to build, when the project has more than one |
| `-C, --clean` | Run the clean steps of the project first |
| `-V, --verbose` | Print the full output of the build tool |
| `--android`, `--ios` | Same as the platform words |

```sh
uji build
uji build ios --clean
```

### `uji install [selector...]`

Builds the app, keeps the build in the store, then installs it on each
selected device in parallel and launches it.

- uji builds each platform one time, then installs that build on every
  device of the platform.
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
| `-A`, `-C`, `-V` | Same as `uji build` |

```sh
uji install
uji install android
uji install "iPhone 17 Pro" fold
uji install -L
uji install -b feature/login
```

If the stored build is from another commit than `HEAD`, or from uncommitted
changes, uji shows a warning and installs it.

### `uji setup [android|ios]`

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
uji deeplink "shop://cart" --android
```

!!! warning
    Put the URL in quotes. If you do not, your shell reads `&` and `?` and
    changes the URL before uji gets it.

uji quotes the URL for the Android device shell, so `&`, `#`, spaces and `'`
arrive unchanged.

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
| none | `./<branch>/<time>.mp4` |
| a directory, or a path that ends in `/` | `<dir>/<branch>/<time>.mp4` |
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

### `uji screenshot [path]`

Saves a PNG of each device screen. The path rules are the same as for
`uji record`, with `.png`.

### `uji launch [selector...]` and `uji uninstall [selector...]`

Launch or uninstall the app of the project. uji takes the application id or
bundle id from the newest stored build. If there is no stored build, uji
takes it from the `id` field of the project file.

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
| `uji builds path <ref>` | Print the path of the APK or `.app` |
| `uji builds prune [--dry-run]` | Remove the expired builds now |

### `uji sync [selector...]`

Copies the stored builds to the ujibox app on each device. See
[Sync and the ujibox app](sync.md).

uji also syncs after each change to the store, unless you turn off
[auto-sync](sync.md#auto-sync).

### `uji config`

| Subcommand | Action |
|---|---|
| `uji config project [--force] [--print]` | Detect the project and write its project file |
| `uji config edit [global]` | Open the project file, or the global config, in `$EDITOR` |
| `uji config show` | Print the project config with the variables filled in |
| `uji config path [global]` | Print the path of the file |
| `uji config alias <name> <device>` | Give a device a short name |
| `uji config auto-sync [on\|off]` | Show, or turn on or off, [auto-sync](sync.md#auto-sync) |

### `uji doctor`

Checks the tools, the config files and the store.

### `uji help [command]` and `uji completion <shell>`

`uji help` prints the help of a command. `uji completion zsh` prints a
completion script. To load it in every zsh session, add this line to
`~/.zshrc`:

```sh
source <(uji completion zsh)
```
