# Contributing to ujibox

## Requirements

- macOS.
- Go 1.26 or later, for `uji`.
- A JDK 17 or later and the Android SDK, for the ujibox app.
- Xcode and [xcodegen](https://github.com/yonaskolb/XcodeGen), for the iOS
  host of the ujibox app.
- Python 3, for the docs and the icons.

## The CLI

```sh
make build   # writes bin/uji
make test    # go vet and go test
```

The code is in `cmd/uji` and `internal/`.

## The ujibox app

The app is in `app/`. It is a Kotlin Multiplatform project with a Compose
Multiplatform UI.

```sh
make app-android   # the Android APK
make app-ios       # the iOS app, for the simulator
make app-test      # the tests of the shared code
```

If Gradle cannot find the Android SDK, set `ANDROID_HOME`, or write
`sdk.dir` in `app/local.properties`.

### Project layout

| Path | Contents |
|---|---|
| `app/shared` | The shared code: data, presenters and UI |
| `app/androidApp` | The Android application |
| `app/iosApp` | The iOS host app; `project.yml` is the xcodegen spec |

The shared code uses these libraries:

| Library | Use |
|---|---|
| [Metro](https://github.com/ZacSweers/metro) | Dependency injection. `AppGraph` is the graph |
| [SQLDelight](https://github.com/sqldelight/sqldelight) | The database that holds the imported index |
| [Molecule](https://github.com/cashapp/molecule) | Runs the presenters. A presenter is a composable function that returns the model of a screen |
| Navigation 3 | The back stack: apps, then the builds of an app, then one build |
| Coroutines and Flow | The screens observe the database through flows |

### Data flow

1. `BuildRepository.refresh()` reads `index.json` from the sync folder.
   `refreshIfChanged()` reads it only if its modification time changed.
2. The repository replaces the rows of the `build` table in one transaction.
3. Each presenter collects a flow of the database and returns a model.
4. The screen draws the model and sends events back through `eventSink`.

If `index.json` does not parse, the app keeps the last builds and shows the
error. If the index has a newer schema, the app tells you to update it.

`uji sync` writes `index.json`. Its format is in
[Sync and the ujibox app](docs/sync.md#indexjson). If you change the
format, change `internal/syncer` and `app/shared/.../data/Index.kt`
together.

### Change the iOS project

Edit `app/iosApp/project.yml`, then generate the Xcode project again:

```sh
cd app/iosApp
xcodegen generate
```

## The logo and the icons

`art/generate_icons.py` draws the logo and writes these files:

- the logo of the repository and of the docs
- the Android launcher icon
- the iOS app icon

Change the shapes or the colors in the script, then run it again. The iOS
icon needs `rsvg-convert` (`brew install librsvg`).

```sh
python3 art/generate_icons.py
```

## The documentation

The docs are in `docs/`, and [Zensical](https://zensical.org) builds them.
`scripts/docs.sh` installs Zensical in `.venv-docs` on the first run, then
runs it.

```sh
scripts/docs.sh         # serve the docs at http://localhost:8000
scripts/docs.sh build   # write the site to site/
```

`make docs-serve` and `make docs` run the same commands.

## Pull requests

- Run `make test`, and `make app-test` if you changed the app.
- Keep one change in one pull request.
- Update the docs when a command, a flag or a file format changes.
