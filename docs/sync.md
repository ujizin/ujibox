# Sync and the ujibox app

The ujibox app runs on a device. It shows the stored builds as a tree, in
the same order as `uji builds`:

```text
Shop
  feature/login
    1.4.0 (140)  3f9a01c2  2h ago
    1.4.0 (139)  77d0e4b1  1d ago
  main
    1.3.2 (132)  a91c0e2f  3d ago
Sample App
  …
```

`uji sync` copies the stored builds into the app storage on each device.

```sh
uji sync               # every running device
uji sync android       # Android devices only
uji sync "Galaxy S24"  # one device
```

## What sync copies

- **To an Android device:** every stored Android build, as APKs.
- **To an iOS simulator:** every stored iOS build, as `.app` bundles.

After a sync, the device has the same builds as the store. If a build left
the store, sync removes it from the device. A second sync copies only the
builds that are new.

!!! warning "iOS cannot install builds from the app"
    Do not expect the ujibox app to install a build on iOS. iOS does not let
    an app install other apps, on a device or on a simulator. On a
    simulator, the ujibox app shows the builds and copies the install
    command. Paste that command in a Mac terminal to install the build.

## Device layout

uji writes into the storage of the ujibox app:

| Platform | Directory |
|---|---|
| Android | `/sdcard/Android/data/<sync_package>/files/ujibox` |
| iOS simulator | `<data container of sync_bundle_id>/Documents/ujibox` |

```text
ujibox/index.json
ujibox/builds/<build-id>/manifest.json
ujibox/builds/<build-id>/<name>.apk      (or <name>.app)
```

Install the ujibox app before the sync; see [Build the ujibox app](app.md).
Set its package and bundle id with `sync_package` and `sync_bundle_id` in the
global config.

### Write order

uji writes in this order, so the app never shows a build that is not
complete:

1. The artifact of each new build.
2. The `manifest.json` of each new build.
3. `index.json`, through a temporary file and a rename.
4. The removal of the builds that are not in the index.

## `index.json`

The app reads only `index.json`. Each build row has all the fields a list
row needs.

```json
{
  "schema": 1,
  "updated_at": "2026-10-22T17:18:25Z",
  "host": "my-mac",
  "platform": "android",
  "apps": [
    {
      "id": "shop",
      "name": "Shop",
      "branches": [
        {
          "name": "feature/login",
          "detached": false,
          "builds": [
            {
              "id": "01M5J9PKMND810HTS059E7SR92",
              "package_id": "com.example.shop.dev",
              "version": { "name": "1.4.0", "code": "140" },
              "sha": "3f9a01c2…",
              "dirty": false,
              "pinned": false,
              "created_at": "2026-10-22T14:30:12Z",
              "size": 84123456,
              "manifest": "builds/01M5J9PKMND810HTS059E7SR92/manifest.json",
              "install": {
                "kind": "apk",
                "file": "builds/01M5J9PKMND810HTS059E7SR92/app-dev-debug.apk"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

- Apps, branches and builds are in order, newest first.
- All paths are relative to the `ujibox` directory.
- On a detached HEAD, the branch `name` is `@<worktree>` and `detached` is
  true.

### `install`

| `kind` | Platform | Fields | What the app does |
|---|---|---|---|
| `apk` | Android | `file` | Installs the APK with the Android PackageInstaller |
| `command` | iOS simulator | `command` | Copies the command to the pasteboard |

An iOS `command` looks like this:

```sh
xcrun simctl install 1A2B3C4D-0000-4000-8000-000000000001 '/Users/me/Library/Developer/CoreSimulator/Devices/…/Documents/ujibox/builds/01M…/Shop Dev.app'
```

The command contains the UDID of the simulator and the path of the synced
copy. It works while that copy exists, until a later sync removes the build.
