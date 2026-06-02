package security

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32              = windows.NewLazySystemDLL("crypt32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procCryptProtectData = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect   = crypt32.NewProc("CryptUnprotectData")
	procLocalFree        = kernel32.NewProc("LocalFree")
)

func ProtectString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data := []byte(value)
	in := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var out dataBlob
	ok, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ok == 0 {
		return "", fmt.Errorf("protect secret with DPAPI: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	protected := unsafe.Slice(out.pbData, out.cbData)
	return base64.StdEncoding.EncodeToString(protected), nil
}

func UnprotectString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode protected secret: %w", err)
	}
	in := dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
	var out dataBlob
	ok, _, callErr := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ok == 0 {
		return "", fmt.Errorf("unprotect secret with DPAPI: %w", callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	plain := unsafe.Slice(out.pbData, out.cbData)
	return string(plain), nil
}
