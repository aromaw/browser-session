package platform

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

type Lock struct {
	h          windows.Handle
	overlapped windows.Overlapped
}

func TryLock(path string) (*Lock, bool, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, false, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return nil, false, e
	}
	var info windows.ByHandleFileInformation
	if e = windows.GetFileInformationByHandle(h, &info); e != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(h)
		return nil, false, errors.New("unsafe lock file")
	}
	l := &Lock{h: h}
	e = windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &l.overlapped)
	if e != nil {
		windows.CloseHandle(h)
		if errors.Is(e, windows.ERROR_LOCK_VIOLATION) {
			return nil, false, nil
		}
		return nil, false, e
	}
	return l, true, nil
}
func (l *Lock) Close() {
	_ = windows.UnlockFileEx(l.h, 0, 1, 0, &l.overlapped)
	_ = windows.CloseHandle(l.h)
}
func Detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
}
func PrepareBrowser(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}
func Replace(src, dst string) error {
	a, e := windows.UTF16PtrFromString(src)
	if e != nil {
		return e
	}
	b, e := windows.UTF16PtrFromString(dst)
	if e != nil {
		return e
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
func SecureDir(path string) error {
	// A protected DACL: current user + SYSTEM. Child profile files inherit it.
	t := windows.GetCurrentProcessToken()
	u, e := t.GetTokenUser()
	if e != nil {
		return e
	}
	sd, e := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + u.User.Sid.String() + ")(A;OICI;FA;;;SY)")
	if e != nil {
		return e
	}
	acl, _, e := sd.DACL()
	if e != nil {
		return e
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

var ntdll = windows.NewLazySystemDLL("ntdll.dll")
var queryProcess = ntdll.NewProc("NtQueryInformationProcess")
var querySystem = ntdll.NewProc("NtQuerySystemInformation")
var user32 = windows.NewLazySystemDLL("user32.dll")
var enumWindows = user32.NewProc("EnumWindows")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var postMessage = user32.NewProc("PostMessageW")

func CloseBrowser(p *os.Process) error {
	// WM_CLOSE lets Chromium flush storage and honor beforeunload dialogs.
	// No taskkill and no forced termination of other browser instances.
	sent := false
	cb := syscall.NewCallback(func(hwnd, unused uintptr) uintptr {
		var pid uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if int(pid) == p.Pid {
			ok, _, _ := postMessage.Call(hwnd, 0x0010, 0, 0)
			if ok != 0 {
				sent = true
			}
		}
		return 1
	})
	enumWindows.Call(cb, 0)
	if !sent {
		return errors.New("no browser window accepted close; quit this session in the browser")
	}
	return nil
}

func commandLine(h windows.Handle) ([]string, error) {
	var need uint32
	queryProcess.Call(uintptr(h), 60, 0, 0, uintptr(unsafe.Pointer(&need)))
	if need == 0 || need > 1<<20 {
		return nil, errors.New("process command line unavailable")
	}
	buf := make([]byte, need)
	status, _, _ := queryProcess.Call(uintptr(h), 60, uintptr(unsafe.Pointer(&buf[0])), uintptr(need), uintptr(unsafe.Pointer(&need)))
	if status != 0 {
		return nil, fmt.Errorf("command line NTSTATUS %x", status)
	}
	// UNICODE_STRING returned by ProcessCommandLineInformation.
	type unicodeString struct {
		Length, MaximumLength uint16
		Buffer                *uint16
	}
	u := (*unicodeString)(unsafe.Pointer(&buf[0]))
	if u.Length == 0 {
		return nil, nil
	}
	start := uintptr(unsafe.Pointer(u.Buffer))
	base := uintptr(unsafe.Pointer(&buf[0]))
	if start < base || start+uintptr(u.Length) > base+uintptr(len(buf)) {
		return nil, errors.New("invalid command line buffer")
	}
	text := windows.UTF16ToString(unsafe.Slice(u.Buffer, int(u.Length)/2))
	return windows.DecomposeCommandLine(text)
}
func snapshot() ([]Process, error) {
	me, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return nil, e
	}
	sid := me.User.Sid.String()
	h, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return nil, e
	}
	defer windows.CloseHandle(h)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	var out []Process
	for e = windows.Process32First(h, &entry); e == nil; e = windows.Process32Next(h, &entry) {
		if entry.ProcessID == 0 {
			continue
		}
		p, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if err != nil {
			name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
			if strings.Contains(name, "chrome") || strings.Contains(name, "chromium") || name == "msedge.exe" || name == "brave.exe" {
				return nil, fmt.Errorf("cannot inspect browser PID %d", entry.ProcessID)
			}
			continue
		}
		var token windows.Token
		err = windows.OpenProcessToken(p, windows.TOKEN_QUERY, &token)
		if err != nil {
			windows.CloseHandle(p)
			return nil, err
		}
		u, err := token.GetTokenUser()
		token.Close()
		if err != nil {
			windows.CloseHandle(p)
			return nil, err
		}
		if u.User.Sid.String() != sid {
			windows.CloseHandle(p)
			continue
		}
		var birth, exit, kernel, user windows.Filetime
		err = windows.GetProcessTimes(p, &birth, &exit, &kernel, &user)
		if err != nil {
			windows.CloseHandle(p)
			return nil, err
		}
		if exit.LowDateTime != 0 || exit.HighDateTime != 0 {
			windows.CloseHandle(p)
			continue
		}
		args, err := commandLine(p)
		windows.CloseHandle(p)
		if err != nil {
			return nil, fmt.Errorf("cannot inspect process %d: %w", entry.ProcessID, err)
		}
		out = append(out, Process{PID: int(entry.ProcessID), Parent: int(entry.ParentProcessID), Birth: fmt.Sprintf("%d:%d", birth.HighDateTime, birth.LowDateTime), Args: args})
	}
	if !errors.Is(e, windows.ERROR_NO_MORE_FILES) {
		return nil, e
	}
	return out, nil
}
func BootID() (string, error) {
	var info struct {
		GUID     windows.GUID
		Firmware uint32
		Flags    uint64
	}
	var size uint32
	status, _, _ := querySystem.Call(90, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return "", fmt.Errorf("boot identity unavailable: %x", status)
	}
	return fmt.Sprint(info.GUID), nil
}
