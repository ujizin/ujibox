---
name: uji
description: Build, install and check an Android or iOS app on emulators, simulators and devices with the uji CLI (ujibox), or build and launch a desktop or web app on this Mac. Use it to run the app after a code change, install a stored build again, open a deeplink, take a screenshot to check the UI, record the screen, or list the devices.
---

# Use uji to run and check a mobile app

`uji` builds the app of the current git checkout, keeps each build in a store
outside the project, and installs it on devices. Run it from the checkout.
The full docs are at https://ujizin.github.io/ujibox/.

## Before you start

1. Check that uji is installed: `uji --version`. If it is not, tell the user
   to install it (`go install github.com/ujizin/ujibox/cmd/uji@latest`).
2. List the devices: `uji devices`. Add `--all` to also see the devices that
   are off.
3. If the project does not build with uji yet, use the `uji-config` skill.

## Run the app after a change

```sh
uji install -A          # build, store, install and launch on Android
uji install -I          # the same on iOS simulators
uji install "Pixel_9"   # one device, by name
```

- uji builds each kind of build one time, then installs it on every device
  that takes it.
- If the build fails, uji prints the lines that explain the failure, a
  hint, and the path of the full log. Read the summary first. Read the log
  only if the summary is not enough.
- `uji install -L` installs the last stored build of this branch again,
  with no build.

## Check the UI

1. Take a screenshot:

    ```sh
    uji screenshot /tmp/uji-check -d "<device>"
    ```

2. Read the PNG file that uji prints, and compare it with what you expect.

To show a flow, record the screen. `uji record` runs until it gets Ctrl-C,
so run it in the background and stop it with SIGINT. Do not send SIGKILL,
or the mp4 file does not play.

```sh
uji record /tmp/uji-check -d "<device>" --no-window &
# … do the flow …
kill -INT %1; wait
```

## Desktop and web apps

Desktop and web apps run on this Mac. uji builds, launches and captures them;
it does not install them.

```sh
uji build desktop && uji launch desktop
uji screenshot desktop /tmp/uji-check
```

`uji launch web` serves the app and opens a Chrome window until it gets
Ctrl-C, so run it in the background:

```sh
uji build web
uji launch web &
uji screenshot web /tmp/uji-check
kill -INT %1; wait
```

- Screenshot and record find only the window of an app that `uji launch`
  started.
- They need the Screen Recording permission of the terminal. If uji reports
  that the terminal cannot capture the screen, tell the user to turn it on.
  Do not try to change the permission yourself.
- `install`, `deeplink`, `sync` and the other device commands reject
  `desktop` and `web`.

## Deeplinks

Put the URL in quotes, because the shell reads `&` and `?`:

```sh
uji deeplink "myapp://orders/42?tab=details" -A
```

## Other commands

| Task | Command |
|---|---|
| Launch the app | `uji launch` (devices), `uji launch desktop`, `uji launch web` |
| Uninstall the app | `uji uninstall` |
| See the stored builds | `uji builds` |
| Build without an install | `uji build` |
| Check the tools | `uji doctor` |

## Rules

- Name a device when more than one is running, so that you do not change a
  device that the user did not mean.
- Do not use a physical iPhone that is only on the network (`device
  (network)` in `uji devices`) unless the user names it. It can belong to
  someone else.
- Do not boot or shut down devices unless the task needs it.
- Use `--no-sync` when the user does not want the ujibox app on the devices
  to change.
