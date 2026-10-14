//go:build !linux

package sandbox

import (
	"fmt"
	"os"
)

func landlockABI() int { return 0 }

// RunHelper només té sentit a Linux.
func RunHelper(args []string) int {
	fmt.Fprintln(os.Stderr, "sandbox helper: Landlock is only available on Linux")
	return 126
}
