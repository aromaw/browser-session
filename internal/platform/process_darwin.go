package platform

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

func snapshot() ([]Process, error) {
	rows, e := unix.SysctlKinfoProcSlice("kern.proc.uid", os.Getuid())
	if e != nil {
		return nil, e
	}
	var out []Process
	for _, k := range rows {
		if k.Proc.P_stat == 5 {
			continue
		} // SZOMB
		pid := int(k.Proc.P_pid)
		raw, e := unix.SysctlRaw("kern.procargs2", pid)
		if e != nil {
			if _, gone := unix.SysctlKinfoProc("kern.proc.pid", pid); gone != nil {
				continue
			}
			return nil, fmt.Errorf("cannot inspect process %d: %w", pid, e)
		}
		var args []string
		if len(raw) >= 4 {
			n := int(binary.LittleEndian.Uint32(raw[:4]))
			data := raw[4:]
			if i := bytes.IndexByte(data, 0); i >= 0 {
				data = bytes.TrimLeft(data[i+1:], "\x00")
				for _, s := range bytes.Split(data, []byte{0}) {
					if len(args) >= n {
						break
					}
					args = append(args, string(s))
				}
			}
		}
		out = append(out, Process{PID: pid, Parent: int(k.Eproc.Ppid), Group: int(k.Eproc.Pgid), Birth: fmt.Sprintf("%d:%d", k.Proc.P_starttime.Sec, k.Proc.P_starttime.Usec), Args: args})
	}
	return out, nil
}
func BootID() (string, error) {
	v, e := unix.SysctlTimeval("kern.boottime")
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%d:%d", v.Sec, v.Usec), nil
}
