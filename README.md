# WANY Keyboard Switcher

Windows US/JIS keyboard-layout selector for Korean, Japanese, and English input. The v0.3 source and Windows x64 release are being prepared.

## Publishing v0.3

Upload `WANY-Keyboard-Switcher-v0.3-source.zip` to the **repository root** on `main` via GitHub's **Add file → Upload files**. The repository's publishing workflow extracts the original source and keyboard icons, commits readable files, runs mapping tests, builds the Windows x64 executable, and publishes the `v0.3` GitHub Release with the EXE and a source ZIP.

The release is not published until the workflow finishes successfully. If GitHub Actions is disabled for this repository or the workflow token lacks write permission, review its Actions run and enable the necessary permissions.

The original archive includes `main.go`, `go.mod`, a complete README, icon assets, mapping tests, and a subsequent tag-triggered release workflow; this provisional README will be replaced automatically after unpacking.
