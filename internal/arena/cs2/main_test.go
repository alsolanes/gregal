package cs2

import (
	"os"
	"runtime/debug"
	"testing"
)

// Les simulacions creen molts objectes petits: menys GC, proves més ràpides.
func TestMain(m *testing.M) {
	debug.SetGCPercent(400)
	os.Exit(m.Run())
}
