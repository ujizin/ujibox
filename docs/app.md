# The ujibox app

The ujibox app runs on Android devices, Android emulators and iOS
simulators. It lists the builds that `uji sync` copies to the device, by app
and branch, and installs them. Its source is in `app/`.

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
