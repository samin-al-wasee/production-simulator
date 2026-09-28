package calibration

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

const (
	procMemInfo   = "/proc/meminfo"
	cgroupMemFile = "/sys/fs/cgroup/memory.max"
	cgroupCPUFile = "/sys/fs/cgroup/cpu.max"
)

// Detect measures the host resources visible to this process. Container
// limits (cgroup v2) take precedence over host totals when present. diskPath
// selects the filesystem whose capacity is reported.
func Detect(diskPath string) (Host, error) {
	f, err := os.Open(procMemInfo)
	if err != nil {
		return Host{}, fmt.Errorf("read memory: %w", err)
	}
	defer f.Close()
	mem, err := ParseMemInfo(f)
	if err != nil {
		return Host{}, fmt.Errorf("read memory: %w", err)
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(diskPath, &st); err != nil {
		return Host{}, fmt.Errorf("read disk %q: %w", diskPath, err)
	}

	cores := float64(runtime.NumCPU())
	if raw, err := os.ReadFile(cgroupCPUFile); err == nil {
		limit, ok, err := ParseCgroupCPU(string(raw))
		if err != nil {
			return Host{}, err
		}
		cores = limitFloat(cores, limit, ok)
	}
	if raw, err := os.ReadFile(cgroupMemFile); err == nil {
		limit, ok, err := ParseCgroupMemory(string(raw))
		if err != nil {
			return Host{}, err
		}
		mem = limitUint(mem, limit, ok)
	}

	return Host{
		CPUCores:    cores,
		MemoryBytes: mem,
		DiskBytes:   st.Blocks * uint64(st.Bsize),
	}, nil
}
