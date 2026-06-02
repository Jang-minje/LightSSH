//go:build !windows

package security

import "errors"

func ProtectString(string) (string, error) {
	return "", errors.New("secure password storage is only implemented on Windows")
}

func UnprotectString(string) (string, error) {
	return "", errors.New("secure password storage is only implemented on Windows")
}
