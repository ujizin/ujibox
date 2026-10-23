# Build the ujibox app

The ujibox app is in `app/`. It is a Kotlin Multiplatform project with a
Compose Multiplatform UI for Android and iOS.

## Install it on a device

1. Build and install the Android app on every running Android device.

    ```sh
    cd app
    ./gradlew :androidApp:installDebug
    ```

2. Build and install the iOS app on a simulator. uji detects the Xcode
   project in `app/iosApp`.

    ```sh
    uji install ios "iPhone 17 Pro"
    ```

3. Copy the stored builds into the app. You must do this only one time.
   After that, [auto-sync](sync.md#auto-sync) copies each new build.

    ```sh
    uji sync
    ```

If Gradle cannot find the Android SDK, set `ANDROID_HOME`, or write
`sdk.dir` in `app/local.properties`.

The app reads `index.json` again each time it comes to the front. While the
app is open, it checks the file every 2 seconds.

## Install a build from the app

- **Android:** tap the install button of a build. The first time, Android
  opens the "Install unknown apps" setting. Allow ujibox, go back, and tap
  the button again. Android then asks you to confirm the install.
- **iOS simulator:** tap the copy button of a build. Paste the command in a
  Mac terminal.

The app marks the build that it installed last for each package as
`installed`.

On the screen of a build, the commit is a link to its web page, if the
checkout has a known remote. The copy button next to it copies that link,
or the commit sha if there is no link.

## Project layout

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

## Tests

```sh
cd app
./gradlew :shared:testAndroidHostTest
```

## Change the iOS project

Edit `app/iosApp/project.yml`, then generate the Xcode project again:

```sh
cd app/iosApp
xcodegen generate
```
