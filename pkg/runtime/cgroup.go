package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type ResourceLimits struct {
	MemoryMB    int
	CPUShares   int
	CPUQuota    int
	CPUPeriod   int
	PidsLimit   int
}

func ApplyCGroupLimits(pid int, containerID string, limits ResourceLimits) error {
	cgroupPath := filepath.Join("/sys/fs/cgroup", "poqman-"+containerID)

	if err := os.MkdirAll(cgroupPath, 0o755); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("root required for cgroups")
		}
		return fmt.Errorf("create cgroup: %w", err)
	}

	if limits.MemoryMB > 0 {
		memoryPath := filepath.Join(cgroupPath, "memory.max")
		memBytes := strconv.Itoa(limits.MemoryMB * 1024 * 1024)
		os.WriteFile(memoryPath, []byte(memBytes), 0o644)
	}

	if limits.CPUShares > 0 {
		cpuPath := filepath.Join(cgroupPath, "cpu.weight")
		os.WriteFile(cpuPath, []byte(strconv.Itoa(limits.CPUShares)), 0o644)
	}

	if limits.PidsLimit > 0 {
		pidsPath := filepath.Join(cgroupPath, "pids.max")
		os.WriteFile(pidsPath, []byte(strconv.Itoa(limits.PidsLimit)), 0o644)
	}

	cgroupProcs := filepath.Join(cgroupPath, "cgroup.procs")
	return os.WriteFile(cgroupProcs, []byte(strconv.Itoa(pid)), 0o644)
}

func RemoveCGroup(containerID string) error {
	cgroupPath := filepath.Join("/sys/fs/cgroup", "poqman-"+containerID)
	return os.RemoveAll(cgroupPath)
}
