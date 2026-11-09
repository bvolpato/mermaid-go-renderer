//go:build !unix

package mermaid

import "time"

// processCPUTime is not measured on this system.
func processCPUTime() time.Duration { return 0 }
