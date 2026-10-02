package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Model defaults are shared by all conversations for one user. A process-wide
// lock protects the read/modify/write cycle when several web sessions select a
// model at the same time.
var modelDefaultsMu sync.Mutex

func modelDefaultsPath(user string) string {
	// Hash the identity rather than slugifying it: distinct usernames such as
	// "ana maria" and "ana-maria" must never share model preferences.
	identity := strings.TrimSpace(user)
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(baseDataDir(), "model-preferences", hex.EncodeToString(sum[:])+".json")
}

// LoadModelDefaults returns the most recently selected model for each role
// for a user. The returned map is independent of the stored data.
func LoadModelDefaults(user string) (map[string]string, error) {
	modelDefaultsMu.Lock()
	defer modelDefaultsMu.Unlock()
	return readModelDefaults(modelDefaultsPath(user))
}

// SaveModelDefault updates one role's default model for a user, preserving
// choices for the user's other roles.
func SaveModelDefault(user, role, selection string) error {
	role = strings.TrimSpace(role)
	selection = strings.TrimSpace(selection)
	if role == "" || selection == "" {
		return fmt.Errorf("session: buit rol o model per defecte")
	}
	modelDefaultsMu.Lock()
	defer modelDefaultsMu.Unlock()

	path := modelDefaultsPath(user)
	defaults, err := readModelDefaults(path)
	if err != nil {
		return err
	}
	defaults[role] = selection
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// Windows cannot rename over an existing file. Preserve the same
		// replacement strategy used by SaveSession.
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}

func readModelDefaults(path string) (map[string]string, error) {
	defaults := map[string]string{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &defaults); err != nil {
		return nil, fmt.Errorf("session: preferències de model corruptes: %w", err)
	}
	if defaults == nil {
		defaults = map[string]string{}
	}
	return defaults, nil
}
