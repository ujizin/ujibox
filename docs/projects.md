# Project files

A project file tells uji how to build the apps of a repository. Each file is
on your machine only, in `~/.config/ujibox/projects/<name>.toml`. uji does
not add files to your repository.

## Make a project file

Run this command in the checkout:

```sh
uji config project
```

uji looks for these items:

- **Gradle application modules.** A module that applies the Android
  application plugin, or that sets `applicationId`. uji reads the first
  product flavor and its `applicationIdSuffix`.
- **Xcode projects and workspaces.** uji reads the shared schemes. If there
  are none, it asks `xcodebuild -list`. Then it asks `xcodebuild
  -showBuildSettings` for the bundle id and the product name.
- **`local.properties`.** If the main checkout has one, uji adds a setup step
  that copies it into each new worktree.

Each value in the file has a comment that tells where it came from. Read the
file, then correct it:

```sh
uji config edit
uji config show
```

`uji config show` prints the result with the variables filled in.

## How uji finds the file

A project file has a `[match]` table. uji compares it with the checkout of
the working directory:

- `remote`: the `origin` remote. uji compares remotes in one form, so
  `git@github.com:org/repo.git` and `https://github.com/org/repo` are equal.
- `paths`: globs of checkout roots. They can start with `~`.

Every worktree of a repository has the same `origin`. Thus one file serves
all worktrees.

If no file matches, uji detects the project each time it runs.

## Format

```toml
name = "shop"
default_app = "shop"

[match]
remote = "github.com/example/shop"

[apps.shop]
display = "Shop"

[apps.shop.android]
id = "com.example.shop.dev"
build = ["./gradlew :app:assembleDevDebug"]
artifact = "app/build/outputs/apk/dev/debug/*.apk"

[apps.shop.ios]
id = "com.example.shop.dev"
build = [
  'xcodebuild -project ios/Shop.xcodeproj -scheme Shop -configuration Debug -destination "generic/platform=iOS Simulator" -derivedDataPath "{derived_data}" build',
]
artifact = "{derived_data}/Build/Products/Debug-iphonesimulator/Shop.app"
```

### Top-level keys

| Key | Meaning |
|---|---|
| `name` | The project name. The default is the file name |
| `default_app` | The app to use when you do not give `--app` |
| `[match]` | `remote` and `paths`, as above |
| `[apps.<key>]` | One table for each app |

### App keys

| Key | Meaning |
|---|---|
| `display` | The name in the output and in the ujibox app |
| `env` | Environment variables for every step of the app |
| `setup`, `clean`, `retry` | Steps for both platforms. See below |
| `[apps.<key>.android]` | How to build the Android app |
| `[apps.<key>.ios]` | How to build the iOS app |

### Platform keys

| Key | Meaning |
|---|---|
| `id` | The application id or bundle id. uji uses it for `launch` and `uninstall` when it has no stored build |
| `build` | The shell commands that make the artifact. They run in order, from the checkout root |
| `artifact` | A glob that finds the APK or `.app` after the build. `**` matches any number of directories. If more than one file matches, uji takes the newest |
| `env` | Environment variables for the build of this platform |
| `setup`, `clean`, `retry` | Steps for this platform only |

After the build, uji reads the real application id and version from the
artifact. Thus the `id` field can omit flavor and build-type suffixes.

## Variables

You can use these variables in `build`, `artifact`, `run`, `then`, `creates`
and `env` values. uji also exports each one to the commands as
`UJI_<NAME>`, for example `UJI_ROOT`.

| Variable | Value |
|---|---|
| `{root}` | The checkout root |
| `{main_root}` | The main checkout of the repository. In a linked worktree, this is the original checkout |
| `{derived_data}` | A DerivedData directory for this checkout, in `~/Library/Caches/ujibox` |
| `{branch}` | The branch, or `@<worktree>` on a detached HEAD |
| `{worktree}` | The name of the checkout directory |
| `{app}` | The app key |
| `{platform}` | `android` or `ios` |

uji does not quote the values. If a path can contain spaces, put the variable
in quotes: `"{derived_data}"`.

## Steps

### Setup

Setup steps run before each build. Use them to prepare a checkout.

```toml
[[apps.shop.android.setup]]
run = 'cp "{main_root}/local.properties" "{root}/local.properties"'
creates = "local.properties"

[[apps.shop.ios.setup]]
run = "./scripts/generate-xcode-project.sh"
```

If `creates` names a file that exists, uji skips the step. Use it for a step
that a checkout needs only one time.

### Clean

Clean steps run only with `--clean`. Use them for caches that break.

```toml
[[apps.shop.ios.clean]]
run = 'rm -rf "{derived_data}/SourcePackages/checkouts"'
```

### Retry

If a build fails and its output matches `match`, uji runs `then` and builds
again. uji retries only one time.

```toml
[[apps.shop.android.retry]]
match = 'SDK location not found|local\.properties'
then = 'cp "{main_root}/local.properties" "{root}/local.properties"'
```

`match` is a Go regular expression. Use a literal string (`'...'`), so that
the backslashes stay as they are.

## Example: a Kotlin Multiplatform project

This project has these needs:

- It builds Android through a wrapper script.
- It generates its Xcode project before the build.
- Its Kotlin framework has no x86_64 slice for the simulator.

```toml
name = "market"
default_app = "market"

[match]
remote = "github.com/example/market"

[apps.market]
display = "Market"

[apps.market.android]
id = "com.example.market.debug"
build = ["tools/android-build.sh :androidApp:assembleDebug"]
artifact = "androidApp/build/outputs/apk/debug/*.apk"

[[apps.market.android.setup]]
run = 'cp "{main_root}/local.properties" "{root}/local.properties"'
creates = "local.properties"

[[apps.market.android.retry]]
match = 'SDK location not found'
then = 'cp "{main_root}/local.properties" "{root}/local.properties"'

[apps.market.ios]
id = "com.example.market"
# A generic simulator destination asks for x86_64 too; ARCHS=arm64 limits
# the build to the slice that the Kotlin framework has.
build = [
  'xcodebuild -project iosApp/iosApp.xcodeproj -scheme iosApp -configuration Debug -destination "generic/platform=iOS Simulator" ARCHS=arm64 -derivedDataPath "{derived_data}" build',
]
artifact = "{derived_data}/Build/Products/Debug-iphonesimulator/Market.app"

[[apps.market.ios.setup]]
run = "xcodegen generate --spec iosApp/project.yml"

[[apps.market.ios.clean]]
run = 'rm -rf "{derived_data}"'
```

## The global config

The global config is `~/.config/ujibox/config.toml`. Every key is optional.

```toml
retention_days = 30          # how long an unpinned build stays
keep_last = 3                # builds of each branch and platform that never expire
developer_dir = "/Applications/Xcode.app/Contents/Developer"
android_sdk = "~/Library/Android/sdk"
sync_package = "io.ujibox.app"     # Android package of the ujibox app
sync_bundle_id = "io.ujibox.app"   # bundle id of the ujibox app on simulators

[aliases]
fold = "Pixel_9_Pro_Fold"
```

These environment variables move the uji directories:

| Variable | Default |
|---|---|
| `UJIBOX_HOME` | `~/Library/Application Support/ujibox` (the build store) |
| `UJIBOX_CACHE` | `~/Library/Caches/ujibox` (DerivedData, logs, locks) |
| `UJIBOX_CONFIG` | `~/.config/ujibox` (config files) |
