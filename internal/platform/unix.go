//go:build linux || darwin

package platform

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"syscall"
)

type Lock struct{ f *os.File }

func TryLock(path string) (*Lock, bool, error) {
	fd, e := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, false, e
	}
	f := os.NewFile(uintptr(fd), path)
	e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	if e != nil {
		f.Close()
		if errors.Is(e, unix.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, e
	}
	return &Lock{f}, true, nil
}
func (l *Lock) Close()                 { _ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN); _ = l.f.Close() }
func Detach(c *exec.Cmd)               { c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func PrepareBrowser(c *exec.Cmd)       { c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func CloseBrowser(p *os.Process) error { return p.Signal(syscall.SIGTERM) }
func Replace(src, dst string) error    { return os.Rename(src, dst) }
func SecureDir(path string) error      { return os.Chmod(path, 0700) }
