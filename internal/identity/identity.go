// Package identity names authenticated principals without normalizing their IDs.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
)

// Key is a stable, filesystem-safe, collision-resistant representation of the
// exact authenticated ID. Callers must keep it in a versioned namespace separate
// from legacy slugs: those have no reliable ownership information, even when the
// slug looks like a simple lowercase username (Anna and anna shared one).
func Key(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
