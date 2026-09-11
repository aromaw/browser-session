package main

import (
	"golang.org/x/sys/windows"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

func fakeWindow() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		dll := windows.NewLazySystemDLL("user32.dll")
		create := dll.NewProc("CreateWindowExW")
		peek := dll.NewProc("PeekMessageW")
		dispatch := dll.NewProc("DispatchMessageW")
		isWindow := dll.NewProc("IsWindow")
		cls, _ := syscall.UTF16PtrFromString("STATIC")
		title, _ := syscall.UTF16PtrFromString("browser-session test")
		hwnd, _, _ := create.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 10, 10, 0, 0, 0, 0)
		if hwnd == 0 {
			return
		}
		// MSG includes padding and lPrivate; buffer is deliberately generously sized.
		var msg [64]byte
		for {
			ok, _, _ := isWindow.Call(hwnd)
			if ok == 0 {
				return
			}
			for {
				has, _, _ := peek.Call(uintptr(unsafe.Pointer(&msg[0])), 0, 0, 0, 1)
				if has == 0 {
					break
				}
				dispatch.Call(uintptr(unsafe.Pointer(&msg[0])))
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	return done
}
