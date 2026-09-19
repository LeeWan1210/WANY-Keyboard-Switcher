//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"
	"time"

	"example.com/wany/keyboard-switcher/updatecore"
)

// This value is embedded by the tagged release build with -ldflags "-X main.appVersion=vX.Y".
// GitHub tags, rather than a versioned EXE filename, determine update ordering.
var appVersion = "v0.5"

const (
	WM_UPDATE_RESULT = WM_APP + 3
	ID_CHECK_UPDATE = 110
	MB_OK = 0
	MB_ICONINFORMATION = 0x40
	MB_ICONERROR = 0x10
	MB_YESNO = 0x04
	MB_ICONQUESTION = 0x20
	MB_DEFBUTTON2 = 0x100
	IDYES = 6
)

var (
	pMessageBox = user32.NewProc("MessageBoxW")
	updateBusy bool // owned exclusively by the window/message-loop thread
	updateEvents = make(chan updateEvent, 2)
)

type updateEvent struct {
	release updatecore.Release
	asset updatecore.Asset
	staged string
	manual bool
	upToDate bool
	downloaded bool
	err error
}

func notice(message string, flags uintptr) uintptr {
	r, _, _ := pMessageBox.Call(appWindow, uintptr(unsafe.Pointer(ptr(message))), uintptr(unsafe.Pointer(ptr("WANY Keyboard Switcher"))), flags)
	return r
}

func notifyUpdate(result updateEvent) {
	updateEvents <- result
	pPostMessage.Call(appWindow, WM_UPDATE_RESULT, 0, 0)
}

func updateHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 { return errors.New("too many redirects") }
			if req.URL.Scheme != "https" || req.URL.User != nil {
				return errors.New("unsafe release redirect")
			}
			host := strings.ToLower(req.URL.Hostname())
			if host != "github.com" && host != "api.github.com" &&
				host != "objects.githubusercontent.com" &&
				host != "release-assets.githubusercontent.com" {
				return errors.New("download redirected to an untrusted host")
			}
			return nil
		},
	}
}

func latestRelease() (updatecore.Release, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updatecore.LatestReleaseAPI, nil)
	if err != nil { return updatecore.Release{}, err }
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "WANY-Keyboard-Switcher/"+appVersion)
	res, err := updateHTTPClient(15*time.Second).Do(req)
	if err != nil { return updatecore.Release{}, err }
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return updatecore.Release{}, fmt.Errorf("GitHub API returned HTTP %d", res.StatusCode)
	}
	var release updatecore.Release
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&release); err != nil {
		return updatecore.Release{}, err
	}
	return release, nil
}

// Called only on the UI thread, even when key conversion is paused.
func beginUpdateCheck(manual bool) {
	if updateBusy {
		if manual {
			notice(t("이미 업데이트를 확인하거나 다운로드하는 중입니다.", "An update check or download is already in progress.", "更新の確認またはダウンロードを実行中です。"), MB_OK|MB_ICONINFORMATION)
		}
		return
	}
	updateBusy = true
	go func() {
		ev := updateEvent{manual: manual}
		ev.release, ev.err = latestRelease()
		if ev.err == nil {
			var newer bool
			newer, ev.err = updatecore.IsNewer(appVersion, ev.release.TagName)
			if ev.err == nil {
				if !newer {
					ev.upToDate = true
				} else {
					ev.asset, ev.err = updatecore.SelectUpdate(ev.release)
				}
			}
		}
		notifyUpdate(ev)
	}()
}

func handleUpdateEvent() {
	ev := <-updateEvents
	if ev.downloaded {
		updateBusy = false
		if ev.err != nil {
			notice(t("업데이트 다운로드 또는 검증에 실패했습니다. 기존 프로그램은 그대로 유지됩니다.\n\n", "Update download or verification failed. The current program remains unchanged.\n\n", "更新のダウンロードまたは検証に失敗しました。現在のプログラムはそのまま維持されます。\n\n")+ev.err.Error(), MB_OK|MB_ICONERROR)
			return
		}
		target, err := os.Executable()
		if err != nil {
			notice(err.Error(), MB_OK|MB_ICONERROR)
			return
		}
		// The downloaded EXE starts as a separate updater process. Only after
		// that process starts successfully do we shut down the current instance.
		command := exec.Command(ev.staged, "--apply-update", target, strconv.Itoa(os.Getpid()))
		if err := command.Start(); err != nil {
			notice(t("업데이트 설치 프로그램을 시작하지 못했습니다. 기존 프로그램은 그대로 유지됩니다.\n\n", "Could not start the update installer. The current program remains unchanged.\n\n", "更新用プログラムを開始できませんでした。現在のプログラムはそのまま維持されます。\n\n")+err.Error(), MB_OK|MB_ICONERROR)
			return
		}
		_ = command.Process.Release()
		pDestroyWindow.Call(appWindow)
		return
	}
	updateBusy = false
	if ev.err != nil {
		if ev.manual {
			notice(t("업데이트를 확인하지 못했습니다. 인터넷 연결을 확인한 뒤 다시 시도해 주세요.\n\n", "Could not check for updates. Check your connection and try again.\n\n", "更新を確認できませんでした。接続を確認して再試行してください。\n\n")+ev.err.Error(), MB_OK|MB_ICONERROR)
		}
		return // Startup checks fail silently when offline.
	}
	if ev.upToDate {
		if ev.manual {
			notice(t("현재 최신 버전입니다. (", "You already have the latest version. (", "現在、最新バージョンです。（")+appVersion+")", MB_OK|MB_ICONINFORMATION)
		}
		return
	}
	question := t(
		"새 버전 "+ev.release.TagName+"이(가) 있습니다.\n현재 버전: "+appVersion+"\n\nGitHub 릴리즈에서 다운로드하고 프로그램을 다시 시작할까요?",
		"Version "+ev.release.TagName+" is available.\nInstalled: "+appVersion+"\n\nDownload it from GitHub Releases, replace this executable, and restart?",
		"新しいバージョン "+ev.release.TagName+" が公開されています。\n現在: "+appVersion+"\n\nGitHub Releases からダウンロードし、実行ファイルを更新して再起動しますか？",
	)
	if notice(question, MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2) != IDYES {
		return // Consent required: no download, replacement, or restart.
	}
	updateBusy = true
	go func() {
		staged, err := stageExecutable(ev.asset)
		notifyUpdate(updateEvent{downloaded: true, staged: staged, err: err})
	}()
}

// Download only the exact named GitHub Release asset after explicit confirmation.
// GitHub's published sha256 digest and expected file size are both verified.
func stageExecutable(asset updatecore.Asset) (string, error) {
	directory, err := os.MkdirTemp("", "WANY-Keyboard-Switcher-update-")
	if err != nil { return "", err }
	keep := false
	defer func() { if !keep { _ = os.RemoveAll(directory) } }()
	path := filepath.Join(directory, updatecore.Executable)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil { return "", err }
	defer out.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", "WANY-Keyboard-Switcher/"+appVersion)
	res, err := updateHTTPClient(120*time.Second).Do(req)
	if err != nil { return "", err }
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK { return "", fmt.Errorf("release download returned HTTP %d", res.StatusCode) }
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(out,hash),io.LimitReader(res.Body,asset.Size+1))
	if err != nil { return "", err }
	if count != asset.Size { return "", errors.New("downloaded file size does not match GitHub Release") }
	if !strings.EqualFold("sha256:"+hex.EncodeToString(hash.Sum(nil)),asset.Digest) {
		return "", errors.New("downloaded file failed SHA256 integrity verification")
	}
	if err := out.Sync(); err != nil { return "",err }
	if err := out.Close(); err != nil { return "",err }
	keep = true
	return path,nil
}

func waitForParent(pid uint32) {
	if pid == 0 { return }
	openProcess := kernel32.NewProc("OpenProcess")
	wait := kernel32.NewProc("WaitForSingleObject")
	closeHandle := kernel32.NewProc("CloseHandle")
	const synchronize = 0x00100000
	handle, _, _ := openProcess.Call(synchronize, 0, uintptr(pid))
	if handle != 0 {
		// Allow time for the old GUI process to release its executable image.
		wait.Call(handle, 30000)
		closeHandle.Call(handle)
	}
}

// Applies an update from a downloaded copy of this program after the parent
// exits. On replacement failure the original executable is restored.
func applyUpdate(arguments []string) {
	// Never install to arbitrary names: avoid using this helper as a generic
	// file-overwrite mechanism. The target must be our currently running EXE.
	if len(arguments) != 4 { return }
	target := filepath.Clean(arguments[2])
	name := strings.ToLower(filepath.Base(target))
	if name != strings.ToLower(updatecore.Executable) { return }
	pid, err := strconv.ParseUint(arguments[3],10,32)
	if err != nil || pid == 0 { return }
	load() // Load the user's menu language for any installation error.
	stage, err := os.Executable()
	if err != nil { return }
	waitForParent(uint32(pid))
	backup := fmt.Sprintf("%s.wany-backup-%d",target,time.Now().UnixNano())
	var moveErr error
	for attempt:=0; attempt<40; attempt++ {
		moveErr = os.Rename(target,backup)
		if moveErr == nil { break }
		time.Sleep(250*time.Millisecond)
	}
	if moveErr != nil {
		notice(t("프로그램 파일을 교체하지 못했습니다. 설치 폴더의 권한 또는 실행 중인 프로그램을 확인해 주세요.\n\n","Could not replace the executable. Check folder permissions and whether the old app is still running.\n\n","実行ファイルを置き換えられませんでした。フォルダーの権限と旧アプリの実行状態を確認してください。\n\n")+moveErr.Error(),MB_OK|MB_ICONERROR)
		return
	}
	source,err:=os.Open(stage)
	if err!=nil { _=os.Rename(backup,target); return }
	destination,err:=os.OpenFile(target,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0700)
	if err!=nil {
		source.Close()
		_=os.Rename(backup,target)
		notice(t("새 프로그램 파일을 저장하지 못했습니다. 기존 파일을 복원했습니다.","Could not save the new executable. The old executable was restored.","新しい実行ファイルを保存できませんでした。旧ファイルを復元しました。"),MB_OK|MB_ICONERROR)
		return
	}
	_,copyErr:=io.Copy(destination,source)
	syncErr:=destination.Sync()
	closeErr:=destination.Close()
	source.Close()
	if copyErr!=nil || syncErr!=nil || closeErr!=nil {
		_=os.Remove(target)
		_=os.Rename(backup,target)
		notice(t("업데이트 파일 교체에 실패해 기존 프로그램을 복원했습니다.","Update replacement failed; the original program was restored.","更新に失敗したため、旧プログラムを復元しました。"),MB_OK|MB_ICONERROR)
		return
	}
	if err:=exec.Command(target).Start();err!=nil {
		notice(t("업데이트 파일은 설치됐지만 자동 재시작하지 못했습니다. 프로그램을 직접 실행해 주세요.\n\n","The update was installed but could not restart. Please launch the program manually.\n\n","更新は完了しましたが、自動再起動に失敗しました。手動で起動してください。\n\n")+err.Error(),MB_OK|MB_ICONERROR)
	}
	_=os.Remove(backup)
	// The staged updater executable is still running, so Windows may retain
	// its temporary directory until the next cleanup; no old EXE is removed.
}

