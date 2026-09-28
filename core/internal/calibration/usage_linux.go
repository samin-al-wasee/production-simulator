package calibration

import (
	"fmt"
	"os"
	"syscall"
)

const procLoadAvg = "/proc/loadavg"

// Usage measures current host consumption: CPU as the 1-minute load average
// capped at the detected core count, memory as total minus available, and
// disk as used blocks on diskPath's filesystem.
func Usage(host Host, diskPath string) (Host, error) {
	raw, err := os.ReadFile(procLoadAvg)
	if err != nil {
		return Host{}, fmt.Errorf("read load: %w", err)
	}
	load, err := ParseLoadAvg(string(raw))
	if err != nil {
		return Host{}, err
	}
	if load > host.CPUCores {
		load = host.CPUCores
	}

	f, err := os.Open(procMemInfo)
	if err != nil {
		return Host{}, fmt.Errorf("read memory: %w", err)
	}
	defer f.Close()
	avail, err := ParseMemAvailable(f)
	if err != nil {
		return Host{}, err
	}
	var usedMem uint64
	if avail < host.MemoryBytes {
		usedMem = host.MemoryBytes - avail
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(diskPath, &st); err != nil {
		return Host{}, fmt.Errorf("read disk %q: %w", diskPath, err)
	}
	usedDisk := (st.Blocks - st.Bfree) * uint64(st.Bsize)

	return Host{CPUCores: load, MemoryBytes: usedMem, DiskBytes: usedDisk}, nil
}
