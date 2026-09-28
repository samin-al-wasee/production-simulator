//go:build !linux

package calibration

import "errors"

// Detect is only implemented on Linux, the platform of the ForgeLab runtime
// (devcontainer and Docker hosts).
func Detect(diskPath string) (Host, error) {
	return Host{}, errors.New("hardware detection is only supported on linux")
}
