package tools

import "testing"

func TestIsRooted(t *testing.T) {
	casos := map[string]bool{
		"main.go":             false,
		"sub/dir/f.go":        false,
		"./x":                 false,
		"../fora.go":          false, // escapa, però no per ser arrel
		"":                    false,
		"/etc/passwd":         true,
		"/":                   true,
		`\Windows\System32`:   true, // arrel sense unitat: filepath.IsAbs hi diu que no
		`C:\Users\x`:          true,
		"C:fitxer":            true, // relatiu al dir actual d'una altra unitat
		`\\servidor\recurs\x`: true, // UNC
		"c:/users/x":          true,
	}
	for p, want := range casos {
		if got := IsRooted(p); got != want {
			t.Errorf("IsRooted(%q) = %v, volia %v", p, got, want)
		}
	}
}
