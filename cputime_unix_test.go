//go:build unix

package mermaid

import (
	"syscall"
	"time"
)

// processCPUTime is the CPU time that this process has used: user and
// system, of all threads. It counts the garbage collector, which runs on
// other threads than the render.
func processCPUTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
