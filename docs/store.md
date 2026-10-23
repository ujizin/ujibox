# Build store

uji keeps each build in a store outside the project. The store is in
`~/Library/Application Support/ujibox`. Set `UJIBOX_HOME` to move it.

uji does not use `/tmp` or `~/Library/Caches` for builds. macOS removes
files from those directories, and cleaner tools empty them.

## Layout

```text
apps/<app-id>/builds/<build-id>/manifest.json
apps/<app-id>/builds/<build-id>/build.log
apps/<app-id>/builds/<build-id>/<name>.apk      (or <name>.app)
tmp/                                            builds that are in progress
locks/
```

- `<app-id>` is `<project>.<app>`, or the project name when the two are
  equal. Thus two projects can each have an app called `sample`.
- `<build-id>` is a ULID: it sorts by time and is safe in every path.
- A branch name is never part of a path. It is in the manifest only. Thus a
  `/` in a branch name causes no problem.

uji copies the artifact with an APFS clone (`cp -c`). On APFS, a stored build
uses almost no extra disk space while the original file exists.

## How a build gets into the store

1. uji takes a lock on the checkout. A second `uji build` in the same
   checkout waits. Builds in different worktrees run at the same time.
2. uji runs the setup steps, then the build commands.
3. uji finds the artifact and checks it:
    - An APK must be a valid zip with an `AndroidManifest.xml`.
    - An `.app` must have an `Info.plist` with a bundle id, and its
      executable. It must be a simulator build.
4. uji reads the application id and the version from the artifact.
5. uji copies the artifact and the manifest into `tmp/`, then moves the
   directory into place in one rename.

If the build stops before step 5, the store does not change.

## Groups

uji groups builds by app, then by branch. On a detached HEAD, the group is
`@<worktree>`, the name of the checkout directory.

## Refs

`uji install -b`, `uji builds pin`, `rm` and `path` take a ref:

| Ref | Build |
|---|---|
| `last` | The newest build of this branch |
| `2` | The second newest build of this branch, as `uji builds` numbers it |
| `p8x1ve` | The short id that `uji builds` shows |
| `01JBX7…` | The full build id |
| `3f9a01c2` | The newest build of that commit (4 or more hex digits) |
| `feature/login` | The newest build of that branch |

## Retention

A build expires after 30 days. These builds stay:

- A pinned build (`uji builds pin <ref>`).
- The newest 3 builds of each app, platform and branch.

uji removes expired builds after a build, and at most one time each day.
Run `uji builds prune` to remove them now. Change the period with
`retention_days` and `keep_last` in the global config.

uji does not remove a build while another uji command installs or syncs it.

## The manifest

Each build has a `manifest.json`. The ujibox app reads this format. A change
that breaks readers increments `schema`.

```json
{
  "schema": 1,
  "id": "01M5J9PKMND810HTS059E7SR92",
  "app": { "id": "shop", "name": "Shop", "project": "shop" },
  "platform": "android",
  "target": "emulator",
  "package_id": "com.example.shop.dev",
  "version": { "name": "1.4.0", "code": "140" },
  "git": {
    "branch": "feature/login",
    "detached": false,
    "worktree": "shop-login",
    "worktree_path": "/Users/me/code/shop-login",
    "sha": "3f9a01c2…",
    "dirty": true,
    "dirty_hash": "9b1c…",
    "remote": "github.com/example/shop"
  },
  "artifact": { "file": "app-dev-debug.apk", "size": 84123456, "sha256": "…" },
  "build": {
    "commands": ["./gradlew :app:assembleDevDebug"],
    "duration_ms": 72000,
    "host": "my-mac",
    "tool": "uji 0.1.0"
  },
  "created_at": "2026-10-22T14:30:12Z",
  "pinned": false
}
```

- `dirty` is true when the checkout had uncommitted changes to tracked
  files. `dirty_hash` is a hash of `git diff HEAD`. Two dirty builds of one
  commit have different hashes when their changes differ.
- `target` is `emulator` for an APK and `simulator` for an `.app`.
- `artifact.sha256` is set for an APK only.
