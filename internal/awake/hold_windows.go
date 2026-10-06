package awake

import (
	"runtime"
	"syscall"
)

const (
	esSystemRequired = 0x00000001
	esContinuous     = 0x80000000
)

var setThreadExecutionState = syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// takeHold asks Windows to keep the system awake from a thread of its own,
// whose state it is, and clears it on that thread at release.
func takeHold() (func(), error) {
	if err := setThreadExecutionState.Find(); err != nil {
		return nil, err
	}
	done, set := make(chan struct{}), make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if r, _, err := setThreadExecutionState.Call(esContinuous | esSystemRequired); r == 0 {
			set <- err
			return
		}
		set <- nil
		<-done
		setThreadExecutionState.Call(esContinuous)
	}()
	if err := <-set; err != nil {
		return nil, err
	}
	return func() { close(done) }, nil
}
