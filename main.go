//go:build windows

// WANY Keyboard Layout Switcher (Windows x64)
// Native tray + low-level keyboard hook. No AutoHotkey or third-party drivers.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	WM_DESTROY        = 0x0002
	WM_COMMAND        = 0x0111
	WM_LBUTTONUP      = 0x0202
	WM_RBUTTONUP      = 0x0205
	WM_KEYDOWN        = 0x0100
	WM_KEYUP          = 0x0101
	WM_SYSKEYDOWN     = 0x0104
	WM_SYSKEYUP       = 0x0105
	WM_APP            = 0x8000
	WM_TRAY           = WM_APP + 1
	WM_USER           = 0x0400
	WH_KEYBOARD_LL    = 13
	LLKHF_INJECTED    = 0x10
	NIM_ADD           = 0
	NIM_MODIFY        = 1
	NIM_DELETE        = 2
	NIF_MESSAGE       = 1
	NIF_ICON          = 2
	NIF_TIP           = 4
	MF_STRING         = 0
	MF_POPUP          = 0x10
	MF_SEPARATOR      = 0x800
	MF_CHECKED        = 8
	MF_DISABLED       = 2
	TPM_RIGHTBUTTON   = 2
	TPM_NONOTIFY      = 0x80
	KEYEVENTF_KEYUP   = 2
	KEYEVENTF_UNICODE = 4
	VK_SHIFT          = 0x10
	VK_CONTROL        = 0x11
	VK_MENU           = 0x12
	VK_LWIN           = 0x5B
	VK_RWIN           = 0x5C
	IDC_ARROW         = 32512
	IDI_APPLICATION   = 32512
	ID_US             = 101
	ID_JIS            = 102
	ID_BASEJP         = 103
	ID_EXIT           = 104
	ID_LANG_KO        = 106
	ID_LANG_EN        = 107
	ID_LANG_JA        = 108
	ID_TOGGLE_ENABLED = 109
	LANG_ENGLISH      = 0x09
	WM_SENDFEEDBACK   = WM_APP + 2
	LANG_KOREAN       = 0x12
	LANG_JAPANESE     = 0x11
)

type point struct{ X, Y int32 }
type msg struct {
	Hwnd           uintptr
	Message        uint32
	_              uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	LPrivate       uint32
}
type wndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type notifyIconData struct {
	Size                uint32
	_                   uint32
	Hwnd                uintptr
	ID, Flags, Callback uint32
	_                   uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                [16]byte
	BalloonIcon         uintptr
}
type kbdLL struct {
	VkCode, ScanCode, Flags, Time uint32
	ExtraInfo                     uintptr
}
type keybdInput struct {
	Vk, Scan    uint16
	Flags, Time uint32
	ExtraInfo   uintptr
}

// Windows x64 INPUT is 40 bytes (union is sized for MOUSEINPUT), not 32.
// v0.1 used 32 bytes, causing SendInput ERROR_INVALID_PARAMETER and missing keys.
type input struct {
	Type uint32
	_    uint32
	Ki   keybdInput
	_    [8]byte
}
type settings struct {
	Disabled         bool   `json:"disabled,omitempty"`
	Profile          string `json:"profile"`
	JapaneseBaseline string `json:"japanese_baseline"`
	UILanguage       string `json:"ui_language"`
}

var (
	user32                                    = syscall.NewLazyDLL("user32.dll")
	shell32                                   = syscall.NewLazyDLL("shell32.dll")
	kernel32                                  = syscall.NewLazyDLL("kernel32.dll")
	pRegisterClass                            = user32.NewProc("RegisterClassExW")
	pCreateWindow                             = user32.NewProc("CreateWindowExW")
	pDefWindow                                = user32.NewProc("DefWindowProcW")
	pDestroyWindow                            = user32.NewProc("DestroyWindow")
	pGetMessage                               = user32.NewProc("GetMessageW")
	pTranslateMessage                         = user32.NewProc("TranslateMessage")
	pDispatchMessage                          = user32.NewProc("DispatchMessageW")
	pPostQuit                                 = user32.NewProc("PostQuitMessage")
	pLoadIcon                                 = user32.NewProc("LoadIconW")
	pLoadCursor                               = user32.NewProc("LoadCursorW")
	pCreateMenu                               = user32.NewProc("CreatePopupMenu")
	pAppendMenu                               = user32.NewProc("AppendMenuW")
	pTrackMenu                                = user32.NewProc("TrackPopupMenu")
	pDestroyMenu                              = user32.NewProc("DestroyMenu")
	pSetForeground                            = user32.NewProc("SetForegroundWindow")
	pGetCursorPos                             = user32.NewProc("GetCursorPos")
	pPostMessage                              = user32.NewProc("PostMessageW")
	pHook                                     = user32.NewProc("SetWindowsHookExW")
	pUnhook                                   = user32.NewProc("UnhookWindowsHookEx")
	pCallNext                                 = user32.NewProc("CallNextHookEx")
	pAsyncKey                                 = user32.NewProc("GetAsyncKeyState")
	pSendInput                                = user32.NewProc("SendInput")
	pGetForeground                            = user32.NewProc("GetForegroundWindow")
	pGetWindowThread                          = user32.NewProc("GetWindowThreadProcessId")
	pGetLayout                                = user32.NewProc("GetKeyboardLayout")
	pNotify                                   = shell32.NewProc("Shell_NotifyIconW")
	pGetModule                                = kernel32.NewProc("GetModuleHandleW")
	pCreateIcon                               = user32.NewProc("CreateIcon")
	pDestroyIcon                              = user32.NewProc("DestroyIcon")
	pGetKeyState                              = user32.NewProc("GetKeyState")
	appWindow, hook, appIcon, iconUS, iconJIS, iconDisabled uintptr
	hookCallback, windowCallback              uintptr
	conf                                      = settings{Profile: "US", JapaneseBaseline: "JIS", UILanguage: "ko"}
	confPath                                  string
	intercepted                               = map[uint32]bool{}
	//go:embed icon_us.png
	iconDataUS []byte
	//go:embed icon_jis.png
	iconDataJIS []byte
	//go:embed icon_app.png
	iconDataApp []byte
)

func ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func writeTip(n *notifyIconData, s string) {
	for i := range n.Tip {
		n.Tip[i] = 0
	}
	w, _ := syscall.UTF16FromString(s)
	copy(n.Tip[:], w)
}
func makeIcon(pngData []byte, disabled bool) uintptr {
	im, e := png.Decode(bytes.NewReader(pngData))
	if e != nil {
		return 0
	}
	// CreateIcon takes an AND mask and BGRA XOR bitmap, 32 x 32 pixels.
	const n = 32
	andMask := make([]byte, n*n/8)
	xor := make([]byte, n*n*4)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			px, py := x*im.Bounds().Dx()/n, y*im.Bounds().Dy()/n
			r, g, b, a := im.At(px, py).RGBA()
			i := (y*n + x) * 4
			if disabled {
				// Neutral grayscale keyboard icon while the conversion is paused.
				gray := byte((299*(r>>8) + 587*(g>>8) + 114*(b>>8)) / 1000)
				xor[i], xor[i+1], xor[i+2], xor[i+3] = gray, gray, gray, byte(a>>8)
			} else {
				xor[i], xor[i+1], xor[i+2], xor[i+3] = byte(b>>8), byte(g>>8), byte(r>>8), byte(a>>8)
			}
			if a == 0 {
				andMask[y*n/8+x/8] |= byte(0x80 >> uint(x%8))
			}
		}
	}
	if disabled {
		// A recognizable pause symbol inside a ring, over the grayed-out keyboard.
		// A round badge is easier to distinguish from a Roman numeral II.
		const centerX, centerY, radius = 22, 22, 8
		for y := centerY - radius; y <= centerY+radius; y++ {
			for x := centerX - radius; x <= centerX+radius; x++ {
				dx, dy := x-centerX, y-centerY
				distanceSquared := dx*dx + dy*dy
				if distanceSquared > radius*radius {
					continue
				}
				i := (y*n + x) * 4
				switch {
				case (x == 19 || x == 20 || x == 24 || x == 25) && y >= 18 && y <= 26:
					xor[i], xor[i+1], xor[i+2], xor[i+3] = 255, 255, 255, 255
				case distanceSquared >= 49:
					// White outline around the circular pause badge.
					xor[i], xor[i+1], xor[i+2], xor[i+3] = 235, 235, 235, 255
				default:
					xor[i], xor[i+1], xor[i+2], xor[i+3] = 52, 52, 52, 255
				}
				andMask[y*n/8+x/8] &^= byte(0x80 >> uint(x%8))
			}
		}
	}
	h, _, _ := pCreateIcon.Call(0, n, n, 1, 32, uintptr(unsafe.Pointer(&andMask[0])), uintptr(unsafe.Pointer(&xor[0])))
	runtime.KeepAlive(andMask)
	runtime.KeepAlive(xor)
	return h
}
func save() {
	b, _ := json.MarshalIndent(conf, "", "  ")
	_ = os.MkdirAll(filepath.Dir(confPath), 0700)
	_ = os.WriteFile(confPath, b, 0600)
}
func load() {
	dir, e := os.UserConfigDir()
	if e != nil {
		dir = os.TempDir()
	}
	confPath = filepath.Join(dir, "WANYKeyboardSwitcher", "settings.json")
	b, e := os.ReadFile(confPath)
	if e == nil {
		_ = json.Unmarshal(b, &conf)
	}
	if conf.Profile != "JIS" {
		conf.Profile = "US"
	}
	if conf.JapaneseBaseline != "US" {
		conf.JapaneseBaseline = "JIS"
	}
	if conf.UILanguage != "en" && conf.UILanguage != "ja" {
		conf.UILanguage = "ko"
	}
}

// UI strings are bundled in the executable; this changes only visible menu text.
func t(ko, en, ja string) string {
	switch conf.UILanguage {
	case "en":
		return en
	case "ja":
		return ja
	default:
		return ko
	}
}
func tray(op uintptr) {
	var n notifyIconData
	n.Size = uint32(unsafe.Sizeof(n))
	n.Hwnd = appWindow
	n.ID = 1
	n.Flags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	n.Callback = WM_TRAY
	n.Icon = appIcon
	if conf.Disabled {
		writeTip(&n, "WANY Keyboard - "+conf.Profile+" - "+t("일시중지 중 / 클릭: 다시 시작 / 우클릭: 설정", "Paused / click: resume / right click: settings", "一時停止中 / 左クリック: 再開 / 右クリック: 設定"))
	} else {
		writeTip(&n, "WANY Keyboard - "+conf.Profile+" - "+t("클릭: 배열 전환 / 우클릭: 설정", "Click: switch layout / right click: settings", "左クリック: 配列切替 / 右クリック: 設定"))
	}
	pNotify.Call(op, uintptr(unsafe.Pointer(&n)))
}
func updateTrayIcon() {
	if conf.Disabled {
		appIcon = iconDisabled
	} else if conf.Profile == "JIS" {
		appIcon = iconJIS
	} else {
		appIcon = iconUS
	}
	tray(NIM_MODIFY)
}
func setProfile(s string) {
	if conf.Profile != s {
		conf.Profile = s
		save()
		updateTrayIcon()
	}
}
func setDisabled(disabled bool) {
	if conf.Disabled != disabled {
		conf.Disabled = disabled
		save()
		updateTrayIcon()
	}
}
func setUILanguage(v string) { conf.UILanguage = v; save(); tray(NIM_MODIFY) }
func appendMenu(menu uintptr, flags uintptr, id uintptr, label string) {
	pAppendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(ptr(label))))
}
func popMenu() {
	menu, _, _ := pCreateMenu.Call()
	if menu == 0 {
		return
	}
	defer pDestroyMenu.Call(menu)
	usFlag := uintptr(MF_STRING)
	jiFlag := uintptr(MF_STRING)
	jpFlag := uintptr(MF_STRING)
	if conf.Profile == "US" {
		usFlag |= MF_CHECKED
	} else {
		jiFlag |= MF_CHECKED
	}
	if conf.JapaneseBaseline == "JIS" {
		jpFlag |= MF_CHECKED
	}
	appendMenu(menu, usFlag, ID_US, t("US / 한국어 배열 키보드", "US / Korean ANSI keyboard", "US / 韓国語 ANSI キーボード"))
	appendMenu(menu, jiFlag, ID_JIS, t("JIS / 일본어 배열 키보드", "JIS / Japanese keyboard", "JIS / 日本語配列キーボード"))
	pAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
	if conf.Disabled {
		appendMenu(menu, MF_STRING, ID_TOGGLE_ENABLED, t("▶ 키 변환 다시 시작", "▶ Resume key conversion", "▶ キー変換を再開"))
	} else {
		appendMenu(menu, MF_STRING, ID_TOGGLE_ENABLED, t("⏸ 키 변환 일시중지", "⏸ Pause key conversion", "⏸ キー変換を一時停止"))
	}
	pAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
	appendMenu(menu, jpFlag, ID_BASEJP, t("일본어 IME 실제 배열: "+conf.JapaneseBaseline+" (클릭하여 변경)", "Japanese IME Windows layout: "+conf.JapaneseBaseline+" (click to change)", "日本語IMEのWindows配列: "+conf.JapaneseBaseline+" (クリックで変更)"))
	pAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
	// Language changes are rare: keep them under one native Windows submenu.
	languageMenu, _, _ := pCreateMenu.Call()
	if languageMenu != 0 {
		for _, o := range []struct {
			id         uintptr
			name, code string
		}{{ID_LANG_KO, "한국어", "ko"}, {ID_LANG_EN, "English", "en"}, {ID_LANG_JA, "日本語", "ja"}} {
			flags := uintptr(MF_STRING)
			if o.code == conf.UILanguage {
				flags |= MF_CHECKED
			}
			appendMenu(languageMenu, flags, o.id, o.name)
		}
		// The parent menu owns the submenu after AppendMenuW succeeds.
		// DestroyMenu(parent) will then release both menus together.
		appendMenu(menu, MF_POPUP|MF_STRING, languageMenu, t("표시 언어", "Display language", "表示言語"))
	}
	pAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
	checkFlag := uintptr(MF_STRING)
	if updateBusy { checkFlag |= MF_DISABLED }
	appendMenu(menu, checkFlag, ID_CHECK_UPDATE, t("업데이트 확인 ("+appVersion+")", "Check for updates ("+appVersion+")", "更新を確認 ("+appVersion+")"))
	pAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
	appendMenu(menu, MF_STRING, ID_EXIT, t("프로그램 종료", "Exit WANY Keyboard Switcher", "プログラムを終了"))
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForeground.Call(appWindow)
	pTrackMenu.Call(menu, TPM_RIGHTBUTTON, uintptr(int64(pt.X)), uintptr(int64(pt.Y)), 0, appWindow, 0)
	pPostMessage.Call(appWindow, WM_USER, 0, 0)
}
func wndProc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	switch m {
	case WM_TRAY:
		if l == WM_LBUTTONUP {
			if conf.Disabled {
				setDisabled(false)
			} else if conf.Profile == "US" {
				setProfile("JIS")
			} else {
				setProfile("US")
			}
		} else if l == WM_RBUTTONUP {
			popMenu()
		}
		return 0
	case WM_UPDATE_RESULT:
		handleUpdateEvent()
		return 0
	case WM_COMMAND:
		switch uint32(w) & 0xffff {
		case ID_US:
			setProfile("US")
		case ID_JIS:
			setProfile("JIS")
		case ID_TOGGLE_ENABLED:
			setDisabled(!conf.Disabled)
		case ID_BASEJP:
			if conf.JapaneseBaseline == "JIS" {
				conf.JapaneseBaseline = "US"
			} else {
				conf.JapaneseBaseline = "JIS"
			}
			save()
			tray(NIM_MODIFY)
		case ID_CHECK_UPDATE:
			beginUpdateCheck(true)
		case ID_LANG_KO:
			setUILanguage("ko")
		case ID_LANG_EN:
			setUILanguage("en")
		case ID_LANG_JA:
			setUILanguage("ja")
		case ID_EXIT:
			pDestroyWindow.Call(hwnd)
		}
		return 0
	case WM_DESTROY:
		tray(NIM_DELETE)
		if hook != 0 {
			pUnhook.Call(hook)
			hook = 0
		}
		if iconUS != 0 {
			pDestroyIcon.Call(iconUS)
		}
		if iconJIS != 0 {
			pDestroyIcon.Call(iconJIS)
		}
		if iconDisabled != 0 {
			pDestroyIcon.Call(iconDisabled)
		}
		pPostQuit.Call(0)
		return 0
	}
	r, _, _ := pDefWindow.Call(hwnd, uintptr(m), w, l)
	return r
}
func down(vk uintptr) bool { r, _, _ := pAsyncKey.Call(vk); return r&0x8000 != 0 }
func foregroundLanguage() uint16 {
	hwnd, _, _ := pGetForeground.Call()
	if hwnd == 0 {
		return 0
	}
	th, _, _ := pGetWindowThread.Call(hwnd, 0)
	hkl, _, _ := pGetLayout.Call(th)
	return uint16(hkl & 0x3ff)
}

// Scan codes for the standard number row and punctuation locations. These are
// candidate common mappings; actual extra JIS keys vary by hardware/firmware.
func mappedCharacter(sc uint32, shift bool, physical, baseline string) (string, bool) {
	if physical == baseline {
		return "", false
	}
	if physical == "JIS" && baseline == "US" {
		switch sc {
		case 0x03:
			if shift {
				return "\"", true
			}
		case 0x07:
			if shift {
				return "&", true
			}
		case 0x08:
			if shift {
				return "'", true
			}
		case 0x09:
			if shift {
				return "(", true
			}
		case 0x0A:
			if shift {
				return ")", true
			}
		case 0x0B:
			if shift {
				return "", true
			} // JIS Shift+0 prints no character.
		case 0x0C:
			if shift {
				return "=", true
			}
		case 0x0D:
			if shift {
				return "~", true
			}
			return "^", true
		case 0x1A:
			if shift {
				return "`", true
			}
			return "@", true
		case 0x1B:
			if shift {
				return "{", true
			}
			return "[", true
		case 0x27:
			if shift {
				return "+", true
			}
		case 0x28:
			if shift {
				return "*", true
			}
			return ":", true
		case 0x2B:
			if shift {
				return "}", true
			}
			return "]", true
		case 0x73:
			if shift {
				return "_", true
			}
			return "\\", true // JIS Ro key, if firmware supplies SC073
		case 0x7D:
			if shift {
				return "|", true
			}
			return "\\", true // JIS Yen key; often backslash in code
		}
	}
	if physical == "US" && baseline == "JIS" {
		switch sc {
		case 0x03:
			if shift {
				return "@", true
			}
		case 0x07:
			if shift {
				return "^", true
			}
		case 0x08:
			if shift {
				return "&", true
			}
		case 0x09:
			if shift {
				return "*", true
			}
		case 0x0A:
			if shift {
				return "(", true
			}
		case 0x0B:
			if shift {
				return ")", true
			}
		case 0x0C:
			if shift {
				return "_", true
			}
		case 0x0D:
			if shift {
				return "+", true
			}
			return "=", true
		case 0x1A:
			if shift {
				return "{", true
			}
			return "[", true
		case 0x1B:
			if shift {
				return "}", true
			}
			return "]", true
		case 0x27:
			if shift {
				return ":", true
			}
		case 0x28:
			if shift {
				return "\"", true
			}
			return "'", true
		case 0x2B:
			if shift {
				return "|", true
			}
			return "\\", true
		case 0x29:
			if shift {
				return "~", true
			}
			return "`", true // US grave key vs JP 半角/全角
		}
	}
	return "", false
}
func sendUnicode(s string) bool {
	for _, r := range s {
		if r > 0xffff {
			return false
		}
		a := [2]input{
			{Type: 1, Ki: keybdInput{Scan: uint16(r), Flags: KEYEVENTF_UNICODE}},
			{Type: 1, Ki: keybdInput{Scan: uint16(r), Flags: KEYEVENTF_UNICODE | KEYEVENTF_KEYUP}},
		}
		inserted, _, _ := pSendInput.Call(2, uintptr(unsafe.Pointer(&a[0])), unsafe.Sizeof(a[0]))
		if inserted != 2 {
			return false
		}
	}
	return true
}
func keyboardProc(nCode int32, w, l uintptr) uintptr {
	if nCode < 0 {
		r, _, _ := pCallNext.Call(hook, uintptr(nCode), w, l)
		return r
	}
	var k = *(*kbdLL)(unsafe.Pointer(l))
	if k.Flags&LLKHF_INJECTED != 0 {
		r, _, _ := pCallNext.Call(hook, uintptr(nCode), w, l)
		return r
	}
	keyDown := w == WM_KEYDOWN || w == WM_SYSKEYDOWN
	keyUp := w == WM_KEYUP || w == WM_SYSKEYUP
	if !keyDown && !keyUp {
		r, _, _ := pCallNext.Call(hook, uintptr(nCode), w, l)
		return r
	}
	keyID := k.ScanCode
	if k.Flags&1 != 0 {
		keyID |= 0xE000
	}
	if keyUp && intercepted[keyID] {
		delete(intercepted, keyID)
		return 1
	}
	if !keyDown {
		r, _, _ := pCallNext.Call(hook, uintptr(nCode), w, l)
		return r
	}
	// Keep the hook installed while paused so the tray icon remains interactive;
	// pass every new physical key event through unchanged. Previously intercepted
	// key releases were handled above to avoid sending an orphan key-up.
	if conf.Disabled {
		return nextHook(nCode, w, l)
	}
	if intercepted[keyID] {
		return 1
	} // block autorepeat for remapped symbols
	if down(VK_CONTROL) || down(VK_MENU) || down(VK_LWIN) || down(VK_RWIN) {
		r, _, _ := pCallNext.Call(hook, uintptr(nCode), w, l)
		return r
	}
	lang := foregroundLanguage()
	base := "US"
	if lang == LANG_JAPANESE {
		base = conf.JapaneseBaseline
	} else if lang != LANG_KOREAN && lang != LANG_ENGLISH {
		return nextHook(nCode, w, l)
	}
	shift := down(VK_SHIFT)
	text, ok := mappedCharacter(k.ScanCode, shift, conf.Profile, base)
	if !ok {
		return nextHook(nCode, w, l)
	}
	// Only suppress the original key if synthetic text was actually sent.
	// Otherwise v0.1's invisible key-loss problem would recur.
	if text != "" && !sendUnicode(text) {
		return nextHook(nCode, w, l)
	}
	intercepted[keyID] = true
	return 1
}
func nextHook(n int32, w, l uintptr) uintptr {
	r, _, _ := pCallNext.Call(hook, uintptr(n), w, l)
	return r
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--apply-update" {
		applyUpdate(os.Args)
		return
	}
	runtime.LockOSThread()
	load()
	windowCallback = syscall.NewCallback(wndProc)
	hookCallback = syscall.NewCallback(keyboardProc)
	inst, _, _ := pGetModule.Call(0)
	cursor, _, _ := pLoadCursor.Call(0, IDC_ARROW)
	iconUS = makeIcon(iconDataUS, false)
	iconJIS = makeIcon(iconDataJIS, false)
	iconDisabled = makeIcon(iconDataApp, true)
	if conf.Disabled {
		appIcon = iconDisabled
	} else if conf.Profile == "JIS" {
		appIcon = iconJIS
	} else {
		appIcon = iconUS
	}
	if appIcon == 0 {
		appIcon, _, _ = pLoadIcon.Call(0, IDI_APPLICATION)
	}
	if unsafe.Sizeof(input{}) != 40 {
		panic(errors.New("invalid Windows x64 INPUT size"))
	}
	className := ptr("WANYKeyboardSwitcherWindow")
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: windowCallback, Instance: inst, Icon: appIcon, Cursor: cursor, ClassName: className, IconSmall: appIcon}
	atom, _, _ := pRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return
	}
	appWindow, _, _ = pCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(ptr("WANY Keyboard Switcher"))), 0, 0, 0, 0, 0, 0, 0, inst, 0)
	if appWindow == 0 {
		return
	}
	tray(NIM_ADD)
	hook, _, _ = pHook.Call(WH_KEYBOARD_LL, hookCallback, inst, 0)
	if hook == 0 {
		tray(NIM_DELETE)
		return
	}
	// Check GitHub in the background on startup; never download without consent.
	beginUpdateCheck(false)
	var message msg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	if hook != 0 {
		pUnhook.Call(hook)
	}
	tray(NIM_DELETE)
}
