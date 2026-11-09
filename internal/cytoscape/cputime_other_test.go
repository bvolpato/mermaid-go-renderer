//go:build !unix

package cytoscape

import "time"

func processCPU() time.Duration { return 0 }
