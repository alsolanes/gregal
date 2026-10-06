package llm

import (
	"os"
	"testing"
	"time"
)

// Synthetic provider failures need retry coverage, not production-length waits.
func TestMain(m *testing.M) {
	RetryBase, RetryMax, RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	os.Exit(m.Run())
}
