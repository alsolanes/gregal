package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// Petites eines que els jocs feien servir d'internal/web. Són prou curtes
// per tenir-ne una còpia aquí en lloc d'engrandir el que exporta Gregal.

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeSmall llegeix un cos JSON petit (256 KB) i estricte.
func decodeSmall(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

// firstJSONObject retalla el primer objecte {...} d'una resposta del model.
func firstJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

func errConst(msg string) error { return &constErr{msg} }

type constErr struct{ msg string }

func (e *constErr) Error() string { return e.msg }

// subtleCompare compara en temps constant (true = no coincideix).
func subtleCompare(want, got string) bool {
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1
}

// splitModel separa «provider/model».
func splitModel(ov string) (string, string, bool) {
	if i := strings.Index(ov, "/"); i > 0 {
		return ov[:i], ov[i+1:], true
	}
	return "", "", false
}

// bgWork compta la feina de fons dels jocs: els tests l'esperen abans de
// restaurar els ganxos que substitueixen (arenaModel, footballModel…).
var bgWork sync.WaitGroup

// goBackground executa fn en segon pla comptant-la a bgWork.
func goBackground(fn func()) {
	bgWork.Add(1)
	go func() {
		defer bgWork.Done()
		fn()
	}()
}

// waitBackground espera que no quedi cap feina de fons dels jocs.
func waitBackground() { bgWork.Wait() }
