package flow

import (
	"fmt"
	"strings"
)

// State és el que es va passant d'un pas a l'altre: claus i text. Tot és
// text a posta — el que produeix un model és text, i inventar-hi tipus
// només afegeix maneres de fallar.
type State map[string]string

// Cond és la condició d'una fletxa. El llenguatge és curt i tancat a
// posta: si fes falta res més ric, el que toca és un pas d'agent que
// decideixi, no un mini-llenguatge a mig fer que ningú sap depurar.
//
//	(buit)                sempre
//	clau                  la clau té valor (i no és "0" ni "false")
//	!clau                 la clau és buida
//	clau == "text"        igual
//	clau != "text"        diferent
//	clau conté "text"     hi apareix (sense distingir majúscules)
type Cond struct {
	raw  string
	key  string
	op   string // "", "truthy", "empty", "==", "!=", "conte"
	want string
}

// ParseCond llegeix una condició. Error si no s'entén: val més dir-ho en
// desar el graf que descobrir-ho a mitja execució.
func ParseCond(s string) (Cond, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Cond{raw: s}, nil
	}
	for _, op := range []string{"==", "!=", " conté ", " conte "} {
		i := strings.Index(t, op)
		if i < 0 {
			continue
		}
		key := strings.TrimSpace(t[:i])
		want := strings.TrimSpace(t[i+len(op):])
		if key == "" {
			return Cond{}, fmt.Errorf("falta la clau abans de %q", strings.TrimSpace(op))
		}
		want = strings.Trim(want, `"'`)
		o := strings.TrimSpace(op)
		if o == "conté" || o == "conte" {
			o = "conte"
		}
		return Cond{raw: s, key: key, op: o, want: want}, nil
	}
	if strings.HasPrefix(t, "!") {
		k := strings.TrimSpace(t[1:])
		if k == "" {
			return Cond{}, fmt.Errorf("falta la clau després de «!»")
		}
		return Cond{raw: s, key: k, op: "empty"}, nil
	}
	if strings.ContainsAny(t, " \t") {
		return Cond{}, fmt.Errorf("no entenc %q: usa clau, !clau, clau == \"text\", clau != \"text\" o clau conté \"text\"", s)
	}
	return Cond{raw: s, key: t, op: "truthy"}, nil
}

// Eval diu si la fletxa passa amb aquest estat.
func (c Cond) Eval(st State) bool {
	if c.op == "" {
		return true
	}
	v := strings.TrimSpace(st[c.key])
	switch c.op {
	case "truthy":
		return v != "" && v != "0" && !strings.EqualFold(v, "false")
	case "empty":
		return v == "" || v == "0" || strings.EqualFold(v, "false")
	case "==":
		return v == c.want
	case "!=":
		return v != c.want
	case "conte":
		return strings.Contains(strings.ToLower(v), strings.ToLower(c.want))
	}
	return false
}

// String torna la condició tal com es va escriure.
func (c Cond) String() string { return c.raw }
