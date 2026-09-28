//go:build !linux

package calibration

import "errors"

// Usage is only implemented on Linux.
func Usage(host Host, diskPath string) (Host, error) {
	return Host{}, errors.New("usage measurement is only supported on linux")
}
