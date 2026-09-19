# WANY Keyboard Switcher v0.3

A Windows 10/11 (x64) US/JIS keyboard-layout selector for Korean, Japanese and English input. Native Go executable, no AutoHotkey or Interception driver.

## 한국어 사용법

1. **이전 v0.1/v0.2를 종료**하세요. 여러 버전이 동시에 실행되면 키 입력이 서로 충돌할 수 있습니다.
2. `WANY-Keyboard-Switcher-v0.3.exe`를 실행합니다. 작업표시줄 알림 영역(숨겨진 아이콘 `^` 포함)에 키보드 모양의 **US(파랑)** 또는 **JIS(주황)** 아이콘이 나타납니다.
3. 아이콘 **왼쪽 클릭**: US ↔ JIS 프로필 전환. **오른쪽 클릭**: US/JIS 직접 선택, 실제 Windows 일본어 IME 하드웨어 배열(US 또는 JIS) 지정, 표시 언어 선택, 입력 진단 로그, 종료.
4. 메뉴의 `한국어 / English / 日本語`에서 표시 언어를 선택합니다. 기본값은 한국어입니다. 마지막 선택 언어는 `settings.json`에 저장됩니다.
5. TH108 JIS 등 일본어 물리 키보드는 JIS 프로필을 선택하고, US/한국어 ANSI 물리 키보드는 US 프로필을 선택합니다. **두 키보드를 동시에 사용하는 자동 장치별 매핑은 지원하지 않습니다.**
6. 한국어 Windows IME는 기존 **101키 종류 1** 설정을 유지할 수 있습니다. 기존 오른쪽 Alt 한/영 및 오른쪽 Ctrl 한자 동작을 별도로 재매핑하지 않습니다.
7. **일본어 IME의 Windows 하드웨어 키보드 배열과 메뉴의 `일본어 IME 실제 배열` 값이 일치해야 합니다.** 메뉴 선택은 실제 Windows 설정 자체를 바꾸지 않습니다.

## What's new / 更新内容

- Embedded *keyboard illustration* icon for **Explorer's EXE file**, plus separate illustrated US/JIS notification-area icons.
- Localized tray tooltips and context menu: 한국어 / English / 日本語 (default: Korean, user-selectable and persisted).
- Keeps v0.2's key conversion logic (no change to the already-used mapping rules).
- `.rsrc` icon resource is generated reproducibly with the included Python script, using an ordinary Windows PE/COFF resource; tray icons are embedded with `go:embed`.

## Known limitations / 알려진 제한

- Input conversion is based on synthetic Unicode for a subset of symbol keys, *not* a full system-wide layout swap. Effects in Japanese IME composition, elevated windows, password fields, games and shortcut-heavy applications may differ.
- ¥, ろ and Japanese-specific keys need testing with individual keyboards/connection types. Support for Korean-layout keyboards or other hardware has not been verified on Windows.
- USB 2.4GHz and Bluetooth may present different scan codes for some keys; switch the physical-keyboard profile manually.
- This project was cross-compiled on Linux and **not tested as a Windows application here**. Use at your own risk; close the app from the tray menu or Task Manager if any keys behave incorrectly.
- The app does not restore prior Windows-wide `Scancode Map` registry overrides; those should remain removed if they previously made Right Alt act as Convert.

## Build / 빌드

Requires Go 1.23+ (and Python 3 with Pillow **only when regenerating icons**). Checked-in icon assets and `rsrc_windows_amd64.syso` are enough to build without Python.

```sh
go test ./selftest
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui -s -w" -o WANY-Keyboard-Switcher-v0.3.exe .
```

To regenerate icon assets (`make_icons.py` requires Pillow and DejaVuSans-Bold font) and the PE icon resource:

```sh
python make_icons.py
python make_windows_resource.py
```

The generated icon file name `rsrc_windows_amd64.syso` must remain inside this Go package when building the Windows x64 executable.

## Settings / 로그

`%APPDATA%\WANYKeyboardSwitcher\settings.json` preserves profile, Japanese IME baseline and menu language. The optional input diagnostics log is at `%APPDATA%\WANYKeyboardSwitcher\input-diagnostics.log`.
