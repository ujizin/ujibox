# Select devices

Most commands take device selectors. A selector is one of these words:

- `android`: every running Android device and emulator.
- `ios`: every booted simulator.
- `all`: both platforms.
- A device name.

With no selector, a command uses every running device and every booted
simulator. It does not boot a device in this case.

## Device names

uji compares a name with each device in this order:

1. An alias from the global config.
2. The exact serial (Android) or UDID (simulator).
3. The exact name: the AVD name, the model of a phone, or the simulator name.
4. The start of a name, serial or UDID.

If a name matches more than one device, uji stops and lists the matches.
There is one exception: if exactly one of the matches is running, uji uses
that one.

```console
$ uji install "iPhone 17"
✘ "iPhone 17" matches 2 devices; use one of these names:
  iPhone 17@26.5                   off 1A2B3C4D-0000-4000-8000-000000000001
  iPhone 17@27.0                   off 1A2B3C4D-0000-4000-8000-000000000002
```

A simulator name can repeat across iOS versions. Add `@<version>` to select
one: `"iPhone 17@27.0"`.

## Devices that are off

`uji devices --all` also lists the AVDs and simulators that are off. If you
name one of them, uji boots it and waits until it is ready.

```sh
uji install Pixel_9_Pro_Fold
uji boot "iPhone 17 Pro"
```

## Aliases

Give a long name a short alias:

```sh
uji config alias fold Pixel_9_Pro_Fold
uji install fold
```

uji keeps the aliases in the `[aliases]` table of the global config. The
words `android`, `ios` and `all` cannot be aliases.

## States

| State | Meaning | What to do |
|---|---|---|
| `ready` | Accepts installs | Nothing |
| `off` | An AVD or simulator that is not running | Name it, or run `uji boot` |
| `unauthorized` | The phone did not accept this computer | Unlock the phone and accept the USB debugging prompt |
| `offline` | adb lost the device | Reconnect the cable, or run `adb reconnect` |

uji leaves `unauthorized` and `offline` devices out of "every running device".
