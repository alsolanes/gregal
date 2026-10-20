package web

import "path/filepath"

// A work session is not a conversation owner. Key live histories by the
// canonical durable directory (already scoped by sessDir), kind and stable ID.
// Keep the fields separate: concatenated user/session IDs are ambiguous.
type conversationHistoryKey struct {
	dir, kind, id string
}

func historyKey(s *Server, kind, id string) conversationHistoryKey {
	return conversationHistoryKey{canonicalHistoryDir(s.sessDir()), kind, id}
}

// Resolve existing ancestors too: a user's sessions directory may not exist
// until the first saved turn, but its cache identity must not change then.
func canonicalHistoryDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = filepath.Clean(dir)
	}
	ancestor := abs
	var tail []string
	for {
		if resolved, err := filepath.EvalSymlinks(ancestor); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return abs
		}
		tail = append(tail, filepath.Base(ancestor))
		ancestor = parent
	}
}
