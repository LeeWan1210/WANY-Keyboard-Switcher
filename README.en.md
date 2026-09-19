# WANY Keyboard Switcher

**A Windows utility for manually switching between Japanese JIS and US keyboard layouts from the notification area.** It adjusts a subset of symbol-key inputs so that, when using Korean, English, or Japanese input methods, the resulting characters better match the labels on your physical keyboard.

**Documentation:** [한국어](README.md) · [English (current)](README.en.md) · [日本語](README.ja.md)

> Current version: **v0.6** · Windows 10/11, 64-bit · No AutoHotkey or Interception driver installation required  
> **[Download the latest release](https://github.com/LeeWan1210/WANY-Keyboard-Switcher/releases/latest)**

## Features

- **Pause/resume:** Temporarily disable key conversion without exiting. Physical key events pass through unchanged, the notification-area icon turns gray with a pause badge, and the state is saved.
- Click the notification-area icon to manually switch **US ↔ JIS** profiles.
- See a **blue US / orange JIS** keyboard icon for the selected profile. The EXE also has an embedded keyboard icon.
- Adjust a subset of symbol keys in Korean, English, and Japanese input environments.
- Choose Korean, English, or Japanese from the right-click menu's **Display language submenu**. Your language choice is saved.
- The pause command now uses the recognizable **⏸ pause symbol** instead of a Roman-numeral-like mark. A **circular pause badge** overlays the gray keyboard icon while conversion is paused.
- Does not automatically change the Windows keyboard registry or IME hardware-layout settings.

## Installation and use

1. Exit any previously running v0.1/v0.2 instance. Running multiple versions may cause conflicting key interception.
2. Download and run `WANY-Keyboard-Switcher.exe` from the [releases page](https://github.com/LeeWan1210/WANY-Keyboard-Switcher/releases/latest).
3. Look for the keyboard icon in the notification area, including the hidden-icons menu (`^`).
4. **Left-click:** toggle US/JIS while active, or resume conversion when paused. **Right-click:** choose a profile, **⏸ pause/resume conversion**, set the Japanese IME baseline, choose the **Display language submenu**, **check for updates**, or exit.
5. Select **JIS** for a physical Japanese keyboard such as the TH108 JIS, and **US** for a physical US/ANSI keyboard.
6. If you use the Japanese IME, set **Japanese IME Windows layout** in the app menu to match the hardware keyboard layout configured in Windows. This app menu does **not** change the Windows setting.

**While paused:** The application remains running, forwards original keyboard input, and displays a gray keyboard icon with pause bars. Left-click the icon or choose “Resume key conversion” from its right-click menu to resume. The paused state and selected layout persist across restarts.

**Note:** The app does not automatically detect which physical keyboard is connected. Change the profile manually when you switch keyboards.

## Input methods and keyboard settings

- **Update checks:** On startup the app checks GitHub for a newer stable release. It asks before downloading or installing anything; only after approval does it download, verify, replace the EXE, and restart. You can also check manually from the right-click menu.
- **Korean:** You may keep Windows' Korean 101-key Type 1 configuration. The app does not separately remap your existing Right Alt Hangul/English toggle or Right Ctrl Hanja key.
- **English:** A subset of symbol keys is adjusted according to the selected US/JIS profile.
- **Japanese:** The actual Windows hardware layout (101/102 or 106/109) must match the baseline selected in the app. Behavior during IME composition may differ.

## Known limitations

This is an **experimental symbol-key correction utility, not a complete system-wide keyboard-layout replacement**.

- Japanese IME composition and conversion, `¥`, `ろ`, Henkan/Muhenkan, and other Japanese-specific keys still need device- and environment-specific testing.
- Elevated apps, password fields, games, and applications relying heavily on keyboard shortcuts may behave differently from a basic text editor.
- Complete behavior on physical US/Korean keyboards and across different connection modes has not been verified.
- If keys stop working as expected, exit from the notification-area menu or end the process in Task Manager.
- The app does not undo any earlier system-wide Windows `Scancode Map` registry remapping.

## Settings and updates

The selected profile, Japanese IME baseline, and menu language are saved in:

```text
%APPDATA%\WANYKeyboardSwitcher\settings.json
```

The app checks GitHub Releases at startup, but **never downloads or installs an update before you consent**. After approval, it compares the download against GitHub's SHA-256 digest and file size, exits the old instance, replaces the executable, and restarts. If replacement fails, it attempts to restore the original EXE. Auto-replacement may fail in folders where you lack write permission. **To move from v0.4 or earlier to v0.6, install the new EXE manually once.**

## Source and building

The application is written in Go. With the checked-in icon resources, **Go 1.23 or later** is sufficient to build the Windows 64-bit executable. For example, in an environment where Go is installed:

```sh
go test ./selftest
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui -s -w" -o WANY-Keyboard-Switcher.exe .
```

In Windows PowerShell, set environment variables using the appropriate PowerShell syntax. Python and Pillow are needed only when regenerating icon images. Use `make_windows_resource.py` to regenerate the EXE's icon resource, and keep `rsrc_windows_amd64.syso` in the Go package directory for Windows x64 builds.

**Executable and source ZIP:** [v0.6 release](https://github.com/LeeWan1210/WANY-Keyboard-Switcher/releases/tag/v0.6)

## ☕ Support development

This free utility grew out of the frustration of coding with a Japanese keyboard while typing in Korean. If it has helped you, you can **[buy me a coffee](https://buymeacoffee.com/dldhks1234)**. Support is entirely optional; all features remain available without a donation.

<a href="https://buymeacoffee.com/dldhks1234"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy me a coffee" width="180"></a>
