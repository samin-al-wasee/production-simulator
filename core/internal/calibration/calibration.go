// Package calibration detects the physical resources of the host ForgeLab
// runs on. It is the Physical Layer input to the Resource Virtualization
// Engine (ADR-0003). Parsing is pure and deterministic; only Detect touches
// the operating system.
package calibration

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Host describes the physical resources available to ForgeLab. CPUCores is
// fractional because container CPU quotas are.
type Host struct {
	CPUCores    float64 `json:"cpuCores"`
	MemoryBytes uint64  `json:"memoryBytes"`
	DiskBytes   uint64  `json:"diskBytes"`
}

// ParseMemInfo returns MemTotal in bytes from the contents of /proc/meminfo.
func ParseMemInfo(r io.Reader) (uint64, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemTotal %q: %w", fields[1], err)
		}
		return kb * 1024, nil
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("MemTotal not found")
}

// ParseCgroupMemory parses a cgroup v2 memory.max value. ok is false when the
// value is "max", meaning no limit is set.
func ParseCgroupMemory(s string) (limit uint64, ok bool, err error) {
	s = strings.TrimSpace(s)
	if s == "max" {
		return 0, false, nil
	}
	limit, err = strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse memory.max %q: %w", s, err)
	}
	return limit, true, nil
}

// ParseCgroupCPU parses a cgroup v2 cpu.max value ("<quota> <period>" or
// "max <period>") into fractional cores. ok is false when no quota is set.
func ParseCgroupCPU(s string) (cores float64, ok bool, err error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return 0, false, fmt.Errorf("parse cpu.max %q: want \"<quota> <period>\"", strings.TrimSpace(s))
	}
	if fields[0] == "max" {
		return 0, false, nil
	}
	quota, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse cpu.max quota %q: %w", fields[0], err)
	}
	period, err := strconv.ParseFloat(fields[1], 64)
	if err != nil || period <= 0 {
		return 0, false, fmt.Errorf("parse cpu.max period %q", fields[1])
	}
	return quota / period, true, nil
}

// limitFloat and limitUint apply an optional container limit to a detected
// host value: the effective resource is the smaller of the two.
func limitFloat(detected, limit float64, ok bool) float64 {
	if ok && limit < detected {
		return limit
	}
	return detected
}

func limitUint(detected, limit uint64, ok bool) uint64 {
	if ok && limit < detected {
		return limit
	}
	return detected
}

// ParseLoadAvg returns the 1-minute load average from the contents of
// /proc/loadavg.
func ParseLoadAvg(s string) (float64, error) {
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return 0, fmt.Errorf("parse loadavg %q", strings.TrimSpace(s))
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse loadavg %q: %w", fields[0], err)
	}
	return v, nil
}

// ParseMemAvailable returns MemAvailable in bytes from /proc/meminfo.
func ParseMemAvailable(r io.Reader) (uint64, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[0] != "MemAvailable:" {
			continue
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemAvailable %q: %w", fields[1], err)
		}
		return kb * 1024, nil
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("MemAvailable not found")
}
