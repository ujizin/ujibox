---
name: uji-config
description: Write or fix the uji (ujibox) project config of a repository, in .ujibox.toml or ~/.config/ujibox/projects. Use it when `uji build` cannot build the project, when the build needs setup steps, a wrapper script or a retry rule, or when a physical iPhone needs a signed device build.
---

# Write the uji project config

A project config tells uji how to build each app of a repository. The docs
are at https://ujizin.github.io/ujibox/projects/.

## Where the config lives

uji takes the first of these that exists:

1. A file on this machine, in `~/.config/ujibox/projects/<name>.toml`, that
   matches the checkout by its `[match]` remote or path. It overrides the
   keys of `.ujibox.toml`.
2. `.ujibox.toml` in the root of the checkout. The team commits it.
3. Detection, each time uji runs.

Put values that are true for everyone in `.ujibox.toml`. Put values that are
true only on one machine (a JDK path, a signing team) in the file on the
machine.

## Write a first config

```sh
uji config project --repo   # writes .ujibox.toml, to commit
uji config project          # writes a file on this machine only
uji config show             # prints the result, with the variables filled in
```

uji detects the Gradle application modules and the Xcode projects. Each value
has a comment that says where it came from.

## Format

```toml
name = "shop"
default_app = "shop"

[apps.shop]
display = "Shop"

[apps.shop.android]
build = ["./gradlew :app:assembleDevDebug"]
artifact = "app/build/outputs/apk/dev/debug/*.apk"

[apps.shop.ios]
build = ['xcodebuild -project ios/Shop.xcodeproj -scheme Shop -configuration Debug -destination "generic/platform=iOS Simulator" -derivedDataPath "{derived_data}" build']
artifact = "{derived_data}/Build/Products/Debug-iphonesimulator/Shop.app"

[apps.shop.ios.device]
build = ['xcodebuild -project ios/Shop.xcodeproj -scheme Shop -configuration Debug -destination "generic/platform=iOS" -derivedDataPath "{derived_data}" -allowProvisioningUpdates build']
artifact = "{derived_data}/Build/Products/Debug-iphoneos/Shop.app"
```

- `build` runs from the checkout root, in order.
- `artifact` is a glob. `**` matches any number of folders. uji takes the
  newest match.
- Variables: `{root}`, `{main_root}`, `{derived_data}`, `{branch}`,
  `{worktree}`, `{app}`, `{platform}`.

## Steps

```toml
[[apps.shop.ios.setup]]          # runs before each build
run = "./scripts/generate-project.sh"
creates = "ios/Shop.xcodeproj"   # skip the step when this path exists

[[apps.shop.ios.clean]]          # runs with `uji build --clean`
run = 'rm -rf "{derived_data}"'

[[apps.shop.android.retry]]      # if the log matches, run `then` and build again, one time
match = 'SDK location not found'
then = 'cp "{main_root}/local.properties" "{root}/local.properties"'
```

## Check the config

1. Run `uji config show` and read the commands.
2. Run `uji build` for each platform. Fix the config until the build passes
   and uji stores the artifact.
3. If the file on the machine and `.ujibox.toml` both exist, `uji config
   show` prints both. A key in the file on the machine wins.

## Rules

- Do not put secrets, local paths or signing team ids in `.ujibox.toml`. Put
  them in the file on the machine.
- uji rejects unknown keys. Run `uji config show` after each change.
