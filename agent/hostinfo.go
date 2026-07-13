package main

import (
	"bufio"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// memStatsMB is best-effort: it shells out to OS tools already present on
// every v0 target rather than pulling in a Go dependency for host stats.
func memStatsMB() (totalMB, freeMB int) {
	switch runtime.GOOS {
	case "linux":
		return memStatsLinux()
	case "darwin":
		return memStatsDarwin()
	default:
		return 0, 0
	}
}

func memStatsLinux() (totalMB, freeMB int) {
	out, err := exec.Command("cat", "/proc/meminfo").Output()
	if err != nil {
		return 0, 0
	}
	vals := map[string]int{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if key != "MemTotal" && key != "MemAvailable" {
			continue
		}
		if n, err := strconv.Atoi(fields[1]); err == nil { // kB
			vals[key] = n
		}
	}
	return vals["MemTotal"] / 1024, vals["MemAvailable"] / 1024
}

func memStatsDarwin() (totalMB, freeMB int) {
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
			totalMB = int(n / (1024 * 1024))
		}
	}

	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return totalMB, 0
	}

	pageSize := 4096
	var free, inactive int
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "of" && i+1 < len(fields) {
					if n, err := strconv.Atoi(fields[i+1]); err == nil {
						pageSize = n
					}
				}
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(parts[1]), "."))
		if err != nil {
			continue
		}
		switch key {
		case "Pages free":
			free = val
		case "Pages inactive":
			inactive = val
		}
	}
	freeMB = (free + inactive) * pageSize / (1024 * 1024)
	return totalMB, freeMB
}

// detectAccel is a best-effort guess, overridable via -accel.
func detectAccel() string {
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return "cuda"
	}
	if runtime.GOOS == "darwin" {
		return "metal"
	}
	return "cpu"
}
