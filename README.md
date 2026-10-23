<p align="center"><img src="art/logo.svg" alt="ujibox logo" width="160"></p>

<h1 align="center">ujibox</h1>

ujibox (`uji`) builds your Android and iOS app, keeps each build outside the
project, and installs it on emulators, devices and simulators.

```sh
uji install            # build, store and install on every running device
uji install -L         # install the last stored build of this branch again
uji deeplink "shop://cart"
uji record ~/Movies    # Ctrl-C stops; the file name has the branch and the time
uji builds             # the stored builds, by app and branch
uji                    # the interactive screen
```

ujibox does not show logs and does not inspect network traffic.

## Install

```sh
go install github.com/ujizin/ujibox/cmd/uji@latest
uji doctor
```

The host is macOS. Android needs the Android SDK; iOS needs Xcode; Android
recording needs [scrcpy](https://github.com/Genymobile/scrcpy).

## Documentation

The documentation is in [`docs/`](docs/) and is built with
[Zensical](https://zensical.org):

```sh
pip install zensical
zensical serve
```

## The ujibox app

`uji sync` copies the stored builds into the ujibox app on a device. After
that, uji syncs again after each new build (`uji config auto-sync off` turns
this off). The app lists the builds by app and branch, installs an APK on
Android, and copies the install command on an iOS simulator. The app is a Kotlin Multiplatform
project in [`app/`](app/); see [Build the ujibox app](docs/app.md).

## Development

```sh
make build   # bin/uji
make test
make app-android   # the ujibox app for Android
make app-test
```

`art/generate_icons.py` draws the logo and writes the app icons for both
platforms. Change the shapes or colors there, then run it again.
