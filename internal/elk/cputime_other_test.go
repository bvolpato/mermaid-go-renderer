//go:build !unix

package elk

import "time"

func processCPU() time.Duration { return 0 }
