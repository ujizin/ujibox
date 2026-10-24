# Get started

## Requirements

- macOS.
- Go 1.26 or later, to build uji from source.
- For Android: the Android SDK (`adb`, `emulator`, `aapt2`). uji finds it
  through `ANDROID_HOME`, `ANDROID_SDK_ROOT` or `~/Library/Android/sdk`.
- For iOS: Xcode with the simulator runtimes. Physical iPhones need Xcode 15
  or later, and Xcode 27 or later for screenshots and recording.
- For Android recording: [scrcpy](https://github.com/Genymobile/scrcpy).
  Install it with `brew install scrcpy`.

## Install

1. Install Go 1.26 or later (`brew install go`).
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

## First build

1. Go to the checkout of your app.

    ```sh
    cd ~/code/shop
    ```

2. Write a project file. uji looks at the checkout and writes what it finds.

    ```sh
    uji config project
    ```

3. Read the file and correct the build commands if necessary.

    ```sh
    uji config edit
    ```

4. Build and install on every running device.

    ```sh
    uji install
    ```

!!! note
    A simple project builds without a project file. uji then detects the
    Gradle application module and the Xcode project each time it runs.
    `uji config project` saves the result, so you can edit it.

## Daily use

| Task | Command |
|---|---|
| List devices | `uji devices` |
| Build without an install | `uji build` |
| Build and install on Android only | `uji install android` |
| Install the last build again | `uji install -L` |
| Open a deeplink everywhere | `uji deeplink "shop://cart"` |
| Record the screen | `uji record ~/Movies` |
| See the stored builds | `uji builds` |
| Open the interactive screen | `uji` |

## The interactive screen

Run `uji` with no command to open the screen. The left pane lists the
devices. The right pane lists the stored builds of the current project.

| Key | Action |
|---|---|
| ++tab++ | Move between the panes |
| ++space++ | Select a device; with no selection, actions go to every running device |
| ++enter++ or ++i++ | Install the build under the cursor |
| ++shift+i++ | Build and install |
| ++b++ | Build |
| ++d++ | Open a deeplink |
| ++s++ | Take a screenshot |
| ++r++ | Record; press ++ctrl+c++ to stop |
| ++y++ | Sync to the ujibox app |
| ++p++ | Pin or unpin the build |
| ++x++ ++x++ | Remove the build |
| ++q++ | Quit |

The screen runs the same `uji` commands in the terminal. When a command
ends, press ++enter++ to go back to the screen.
