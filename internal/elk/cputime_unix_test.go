//go:build unix

package elk

import (
	"syscall"
	"time"
)

// processCPU is the CPU time of the process, user and system, over all
// threads. On a machine that runs other programs it is a better measure
// than the wall clock, but it includes the garbage collector.
func processCPU() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
