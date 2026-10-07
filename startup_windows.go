//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	startupRegistryKey      = "Software\\Microsoft\\Windows\\CurrentVersion\\Run"
	startupRegistryValue    = "WANY Keyboard Switcher"
	singleInstanceMutexName = "Local\\WANYKeyboardSwitcher.SingleInstance"

	regSZ         = 1
	keyQueryValue = 0x0001
	keySetValue   = 0x0002

	errorSuccess       = 0
	errorFileNotFound  = 2
	errorAlreadyExists = 183
)

var (
	advapi32         = syscall.NewLazyDLL("advapi32.dll")
	pRegOpenKeyEx    = advapi32.NewProc("RegOpenKeyExW")
	pRegCreateKeyEx  = advapi32.NewProc("RegCreateKeyExW")
	pRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	pRegSetValueEx   = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValue  = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey     = advapi32.NewProc("RegCloseKey")

	pCreateMutexSingleInstance = kernel32.NewProc("CreateMutexW")
	pCloseSingleInstanceHandle = kernel32.NewProc("CloseHandle")
	pSetLastErrorSingleInstance = kernel32.NewProc("SetLastError")
	singleInstanceHandle        uintptr
)

// init runs before the tray window and keyboard hook are created, so duplicate
// launches cannot install a second hook. The self-update helper is intentionally
// exempt because it must run while the old app instance is still shutting down.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "--apply-update" {
		return
	}

	name, err := syscall.UTF16PtrFromString(singleInstanceMutexName)
	if err != nil {
		os.Exit(1)
	}

	// CreateMutexW reports an existing named mutex through GetLastError while
	// still returning a valid handle. Clear stale thread error state first.
	pSetLastErrorSingleInstance.Call(0)
	handle, _, lastErr := pCreateMutexSingleInstance.Call(
		0,
		0,
		uintptr(unsafe.Pointer(name)),
	)
	runtime.KeepAlive(name)

	if handle == 0 {
		os.Exit(1)
	}
	if errors.Is(lastErr, syscall.Errno(errorAlreadyExists)) {
		pCloseSingleInstanceHandle.Call(handle)
		os.Exit(0)
	}

	// Keep the mutex handle open for the entire app lifetime. Windows releases
	// it automatically when this process exits or crashes.
	singleInstanceHandle = handle
}

func startupUTF16Ptr(value string) (*uint16, error) {
	return syscall.UTF16PtrFromString(value)
}

func currentStartupCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	if strings.ContainsRune(executable, '"') {
		return "", errors.New("executable path contains an invalid quote")
	}
	return "\"" + filepath.Clean(executable) + "\"", nil
}

func openStartupKey(access uintptr, create bool) (uintptr, error) {
	subkey, err := startupUTF16Ptr(startupRegistryKey)
	if err != nil {
		return 0, err
	}
	var key uintptr
	var status uintptr
	if create {
		var disposition uint32
		status, _, _ = pRegCreateKeyEx.Call(
			uintptr(0x80000001),
			uintptr(unsafe.Pointer(subkey)),
			0,
			0,
			0,
			access,
			0,
			uintptr(unsafe.Pointer(&key)),
			uintptr(unsafe.Pointer(&disposition)),
		)
		runtime.KeepAlive(subkey)
	} else {
		status, _, _ = pRegOpenKeyEx.Call(
			uintptr(0x80000001),
			uintptr(unsafe.Pointer(subkey)),
			0,
			access,
			uintptr(unsafe.Pointer(&key)),
		)
		runtime.KeepAlive(subkey)
	}
	if status == errorFileNotFound {
		return 0, os.ErrNotExist
	}
	if status != errorSuccess {
		return 0, syscall.Errno(status)
	}
	return key, nil
}

func closeStartupKey(key uintptr) {
	if key != 0 {
		pRegCloseKey.Call(key)
	}
}

func readStartupCommand() (string, bool, error) {
	key, err := openStartupKey(keyQueryValue, false)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer closeStartupKey(key)

	name, err := startupUTF16Ptr(startupRegistryValue)
	if err != nil {
		return "", false, err
	}
	var valueType uint32
	var size uint32
	status, _, _ := pRegQueryValueEx.Call(
		key,
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	runtime.KeepAlive(name)
	if status == errorFileNotFound {
		return "", false, nil
	}
	if status != errorSuccess {
		return "", false, syscall.Errno(status)
	}
	if valueType != regSZ {
		return "", false, errors.New("startup registry value is not REG_SZ")
	}
	if size == 0 {
		return "", true, nil
	}

	buffer := make([]uint16, (size+1)/2)
	status, _, _ = pRegQueryValueEx.Call(
		key,
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&valueType)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	runtime.KeepAlive(name)
	runtime.KeepAlive(buffer)
	if status != errorSuccess {
		return "", false, syscall.Errno(status)
	}
	if valueType != regSZ {
		return "", false, errors.New("startup registry value changed type")
	}
	return syscall.UTF16ToString(buffer), true, nil
}

func startupEnabled() (bool, error) {
	expected, err := currentStartupCommand()
	if err != nil {
		return false, err
	}
	actual, exists, err := readStartupCommand()
	if err != nil {
		return false, err
	}
	return exists && strings.EqualFold(strings.TrimSpace(actual), expected), nil
}

func setStartupEnabled(enabled bool) error {
	name, err := startupUTF16Ptr(startupRegistryValue)
	if err != nil {
		return err
	}

	if !enabled {
		key, openErr := openStartupKey(keySetValue, false)
		if errors.Is(openErr, os.ErrNotExist) {
			return nil
		}
		if openErr != nil {
			return openErr
		}
		defer closeStartupKey(key)
		status, _, _ := pRegDeleteValue.Call(key, uintptr(unsafe.Pointer(name)))
		runtime.KeepAlive(name)
		if status == errorFileNotFound || status == errorSuccess {
			return nil
		}
		return syscall.Errno(status)
	}

	command, err := currentStartupCommand()
	if err != nil {
		return err
	}
	data, err := syscall.UTF16FromString(command)
	if err != nil {
		return err
	}
	key, err := openStartupKey(keyQueryValue|keySetValue, true)
	if err != nil {
		return err
	}
	defer closeStartupKey(key)
	status, _, _ := pRegSetValueEx.Call(
		key,
		uintptr(unsafe.Pointer(name)),
		0,
		regSZ,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)*2),
	)
	runtime.KeepAlive(name)
	runtime.KeepAlive(data)
	if status != errorSuccess {
		return syscall.Errno(status)
	}
	return nil
}
