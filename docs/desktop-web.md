# Desktop and web apps

uji also builds the desktop and web apps of a project, and runs them on this
Mac. A desktop or web app is not on a device, so only some commands work with
it:

| Command | Desktop | Web |
|---|---|---|
| `uji build desktop`, `uji build web` | yes | yes |
| `uji launch desktop`, `uji launch web` | yes | yes |
| `uji screenshot desktop`, `uji screenshot web` | yes | yes |
| `uji record desktop`, `uji record web` | yes | yes |
| `uji builds`, `pin`, `rm`, `path` | yes | yes |
| `install`, `uninstall`, `deeplink`, `devices`, `boot`, `shutdown`, `sync` | no | no |

The commands in the last row stop with an error that says this. The ujibox
app on a phone does not get desktop or web builds.

## Configure the platforms

Add a `desktop` table, a `web` table, or both, to the app in the
[project file](projects.md). They take the same keys as `android` and `ios`:
`build`, `artifact`, `id`, `env`, `setup`, `clean` and `retry`.

```toml
[apps.shop.desktop]
build = ["./gradlew :desktopApp:createDistributable"]
artifact = "desktopApp/build/compose/binaries/main/app/*.app"

[apps.shop.web]
id = "shop-web"
build = ["./gradlew :webApp:wasmJsBrowserDistribution"]
artifact = "webApp/build/dist/wasmJs/productionExecutable"
```

- The desktop artifact is a `.app` or a `.jar`.
    - For a `.app`, uji reads the bundle id and the version from
      `Contents/Info.plist`.
    - A `.jar` has no standard place for them. Set `id`; the version stays
      empty.
- The web artifact is the folder with `index.html`. A web build has no id
  and no version. uji shows `id` as the name of the build.

`uji config project` finds these modules in a Gradle build:

- A Compose desktop app: `compose.desktop { application { ... } }`. uji
  builds it with `createDistributable`.
- A Kotlin web app: a `wasmJs` or `js` target with `binaries.executable()`.
  uji builds it with `wasmJsBrowserDistribution` or `jsBrowserDistribution`.

For another build tool, write the tables yourself. Any command works if it
makes a `.app`, a `.jar` or a folder with `index.html`.

## Build

```sh
uji build desktop
uji build web
uji build              # every platform of the app, desktop and web too
```

uji keeps the builds in the [store](store.md), like Android and iOS builds.

## Launch

`uji launch desktop` starts the newest stored desktop build of this branch.
The app runs after uji exits. A new launch stops the copy that uji started
before. The output of the app goes to a log file in the uji cache.

`uji launch web` does these steps:

1. It serves the newest stored web build on a free `localhost` port.
2. It opens the page in a Google Chrome app window, with a profile of its
   own in the uji cache.
3. It keeps the server alive until you press ++ctrl+c++. Then it closes the
   Chrome window.

If Chrome is not installed, uji prints the URL. Open it in any browser. In
that case, `uji screenshot web` and `uji record web` cannot find the window.

| Flag | Meaning |
|---|---|
| `-b, --build <ref>` | Launch another stored build. See [refs](store.md#refs) |
| `--any-branch` | Take the newest build of any branch |
| `--app <name>` | The app, when the project has more than one |

## Screenshot and record

```sh
uji screenshot desktop
uji screenshot web ~/Desktop
uji record desktop
```

uji finds the window of the app that `uji launch` started, and captures only
that window with `screencapture`. If you start the app in another way, uji
does not find it. The file names follow the same rules as for devices: see
[`uji record`](commands.md#uji-record-path).

!!! note
    The terminal needs the Screen Recording permission:

    1. Open System Settings > Privacy & Security > Screen & System Audio
       Recording.
    2. Turn it on for your terminal app.
    3. Quit and open the terminal again.

    `uji doctor` shows if the terminal has the permission.

A screenshot or a recording needs the window on the screen. If the window is
minimized or on another desktop, uji stops with an error.
