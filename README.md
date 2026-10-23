# ujibox

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

`uji sync` copies the stored builds into the ujibox app on a device. The app
lists them by app and branch, installs an APK on Android, and copies the
install command on an iOS simulator. The app is a Kotlin Multiplatform
project in [`app/`](app/); see [Build the ujibox app](docs/app.md).

## Development

```sh
make build   # bin/uji
make test
make app-android   # the ujibox app for Android
make app-test
```
