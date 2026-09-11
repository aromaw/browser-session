package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func snapshot() ([]Process, error) {
	entries, e := os.ReadDir("/proc")
	if e != nil {
		return nil, e
	}
	var out []Process
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil {
			continue
		}
		d := filepath.Join("/proc", entry.Name())
		info, e := os.Stat(d)
		if e != nil {
			continue
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || st.Uid != uint32(os.Getuid()) {
			continue
		}
		raw, e := os.ReadFile(filepath.Join(d, "stat"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, fmt.Errorf("cannot inspect process %d: %w", pid, e)
		}
		end := strings.LastIndex(string(raw), ")")
		if end < 0 {
			return nil, fmt.Errorf("invalid process stat")
		}
		fields := strings.Fields(string(raw)[end+1:])
		if len(fields) < 20 {
			return nil, fmt.Errorf("short process stat")
		}
		if fields[0] == "Z" || fields[0] == "X" {
			continue
		}
		parent, _ := strconv.Atoi(fields[1])
		group, _ := strconv.Atoi(fields[2])
		cmd, e := os.ReadFile(filepath.Join(d, "cmdline"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, fmt.Errorf("cannot inspect process %d arguments: %w", pid, e)
		}
		out = append(out, Process{PID: pid, Parent: parent, Group: group, Birth: fields[19], Args: strings.Split(strings.TrimRight(string(cmd), "\x00"), "\x00")})
	}
	return out, nil
}
func BootID() (string, error) {
	b, e := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(b)), e
}
