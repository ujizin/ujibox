<p align="center"><img src="assets/logo.svg" alt="ujibox logo" width="160"></p>

# ujibox

ujibox (`uji`) is a CLI that helps you build and debug Android, iOS,
desktop and web apps. It builds your app, keeps each build, and runs it on
emulators, simulators, devices or your Mac in one step.

```console
$ uji install
• Shop · project shop · feature/login@3f9a01c2
✔ Shop android 1.4.0 (140) · 84.1 MB · 1m12s · build 7k2m4q
✔ Shop ios 1.4.0 (140) · 61.3 MB · 3m40s · build p8x1ve
✔  Pixel_9_Pro_Fold  android  installed 1.4.0 (140), launched
✔  iPhone 17 Pro     ios      installed 1.4.0 (140), launched
```

## What ujibox does

- Builds the app of the current checkout with the project's own commands.
- Keeps every build in a store outside the project, grouped by app and
  branch. Builds expire after 30 days, unless you pin them.
- Installs a build on many devices in parallel, and reports each device.
- Installs a stored build again without a rebuild: `uji install --last`.
- Opens deeplinks, takes screenshots and records the screen.
- Builds desktop and web apps, launches them on this Mac, and captures their
  windows. See [Desktop and web apps](desktop-web.md).
- Lists, boots and shuts down emulators and simulators.
- Copies the stored builds to the ujibox app on a device: `uji sync`. After
  each new build, uji syncs again by itself.
- Opens an interactive screen: run `uji` with no command.

## Platforms

| Target | Build | Install | Deeplink | Screenshot | Record |
|---|---|---|---|---|---|
| Android emulator | yes | yes | yes | yes | yes (scrcpy) |
| Android device | yes | yes | yes | yes | yes (scrcpy) |
| iOS simulator | yes | yes | yes | yes | yes (simctl) |
| iOS device | yes¹ | yes | yes | yes² | yes² (devicectl) |
| Desktop (this Mac) | yes | launch³ | no | yes⁴ | yes⁴ (screencapture) |
| Web (Chrome) | yes | launch³ | no | yes⁴ | yes⁴ (screencapture) |

¹ A build for a physical iPhone needs code signing.

² Screenshots and recording on a physical iPhone need Xcode 27 or later.

³ uji does not install desktop and web apps. `uji launch` starts the stored
build.

⁴ The terminal needs the Screen Recording permission.

## Requirements

- uji runs on macOS only.
- uji works with any Android project that Gradle builds and any iOS project
  that Xcode builds. The project does not have to be Kotlin Multiplatform.
- uji detects desktop and web apps only in Compose Multiplatform and Kotlin
  web modules. For other projects, write the build commands in the
  [project file](projects.md).

## Next steps

- [Get started](getting-started.md): install uji and run the first build.
- [Project files](projects.md): tell uji how to build a project that needs
  more than `./gradlew assembleDebug`.
