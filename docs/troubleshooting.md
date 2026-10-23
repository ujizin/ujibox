# Troubleshooting

Run `uji doctor` first. It checks the tools, the config files and the store.

When a build fails, uji shows the lines that explain the failure and the path
of the full log. The logs are in `~/Library/Caches/ujibox/logs`.

## Build

**"the build finished, but nothing matches the artifact pattern"**
:   The `artifact` glob is wrong, or the build wrote the file to another
    place. Run `uji config show` to see the glob with the variables filled
    in. Then find the real file and correct the glob.

**"has no Info.plist; the build was interrupted"**
:   A build stopped before it finished the `.app`. Run the build again.

**"is built for iphoneos"**
:   The build made a device app. Use a simulator destination:
    `-destination "generic/platform=iOS Simulator"`.

**"SDK location not found"**
:   Gradle cannot find the Android SDK. The checkout has no
    `local.properties`. Add a setup step that copies it, as
    [Project files](projects.md#setup) shows.

**The Kotlin/Native cache is broken ("is not found in cache")**
:   Add a clean step that removes the cache, then run the build with
    `--clean`.

**"Could not resolve package dependencies … Package.swift doesn't exist"**
:   The Swift package checkouts are empty. Add a clean step that removes
    `{derived_data}/SourcePackages/checkouts`, then run the build with
    `--clean`. uji keeps DerivedData in `~/Library/Caches/ujibox`, not in
    `/tmp`, so the macOS `/tmp` cleanup cannot empty them.

**"The Xcode build system has crashed"**
:   Run the build again. If it fails again, run it with `--clean`.

**"another uji build runs in this checkout; waiting for it"**
:   Two builds in one checkout write the same output files. The second
    build waits for the first. Use a second worktree to build at the same
    time.

## Install

**`INSTALL_FAILED_UPDATE_INCOMPATIBLE`**
:   The device has the app with another signature. Run `uji uninstall`, then
    install again. The uninstall removes the app data.

**`INSTALL_FAILED_NO_MATCHING_ABIS`**
:   The APK has no native libraries for the CPU of the device.

**"no stored ios build of … for main"**
:   `uji install -L` found no build of this branch. Build one, or use
    `--any-branch`.

**"is unauthorized"**
:   Unlock the phone and accept the USB debugging prompt.

## Devices

**"matches 2 devices"**
:   The name is not unique. Use one of the names that uji lists, for example
    `"iPhone 17@27.0"`, or the UDID.

**"no ready android or ios device"**
:   No device is running. Name an AVD or a simulator to boot it, or run
    `uji boot <name>`.

## Recording

**"scrcpy not found"**
:   Install scrcpy: `brew install scrcpy`.

**The video does not play**
:   The recorder did not finish the file. Press ++ctrl+c++ one time only,
    and wait for uji to report the file.

**The iOS video is very short**
:   `simctl` records a frame only when the screen changes. A screen that does
    not change gives a short video.

## Deeplinks

**"no app on … handles this link"**
:   No installed app declares the scheme. Install the app first, and check
    the intent filters (Android) or the URL types (iOS).

**The URL is not complete on the device**
:   Your shell changed the URL. Put it in quotes:
    `uji deeplink "shop://a?b=1&c=2"`.
