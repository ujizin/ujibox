<p align="center"><img src="art/logo.svg" alt="ujibox logo" width="160"></p>

<h1 align="center">ujibox</h1>

ujibox (`uji`) is a CLI that helps you build and debug Android and iOS
apps. It builds your app, keeps each build, and puts it on emulators,
simulators and devices in one step.

```sh
uji install            # build, store and install on every running device
uji install -L         # install the last build of this branch again
uji deeplink "shop://cart"
uji record ~/Movies    # Ctrl-C stops; the file goes in ~/Movies/<branch>/
uji                    # the interactive screen
```

## Features

- Interactive screen: run `uji`
- Build, with the setup steps of your project
- Project config on your machine, or shared in the repo (`.ujibox.toml`)
- Build store by app and branch, kept for 30 days
- Install on many devices at once, or reinstall a stored build
- Deeplinks, screenshots and screen recording
- Boot and shut down emulators and simulators
- The ujibox app: your builds on the device, synced after each build
- Agent skills for Claude Code, Codex and Gemini CLI: `uji add skills`

## Platforms

| Target | Build | Install | Deeplink | Screenshot | Record |
|---|---|---|---|---|---|
| Android emulator | yes | yes | yes | yes | yes |
| Android device | yes | yes | yes | yes | yes |
| iOS simulator | yes | yes | yes | yes | yes |
| iOS device | yes¹ | yes | yes | yes² | yes² |

¹ A build for a physical iPhone needs code signing.

² Screenshots and recording on a physical iPhone need Xcode 27 or later.

The host is macOS.

## Install

1. Install [Go](https://go.dev/dl/) 1.26 or later (`brew install go`).
2. Install uji:

    ```sh
    go install github.com/ujizin/ujibox/cmd/uji@latest
    ```

3. Add the Go bin folder to your `PATH`, so the shell finds `uji`. Skip
   this step if `which uji` already prints a path.

    ```sh
    echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
    source ~/.zshrc
    ```

4. Check the tools:

    ```sh
    uji doctor
    ```

- Android needs the Android SDK.
- iOS needs Xcode.
- Android screen recording needs [scrcpy](https://github.com/Genymobile/scrcpy).

## Documentation

Read the docs at [ujizin.github.io/ujibox](https://ujizin.github.io/ujibox/):
[get started](https://ujizin.github.io/ujibox/getting-started/),
[commands](https://ujizin.github.io/ujibox/commands/) and
[the ujibox app](https://ujizin.github.io/ujibox/app/).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

```
Copyright 2026 ujizin (Lucas Yuji)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
