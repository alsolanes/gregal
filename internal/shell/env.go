package shell

import (
	"os"
	"runtime"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// utf8Env fa que Python escrigui UTF-8 quan la sortida va a una canonada.
// A Windows, sense això, Python fa servir la pàgina de codis ANSI (cp1252):
// «TOT BÉ» arribava com «TOT B�» i el model, que no s'hi refiava, tornava a
// córrer els tests amb `; echo exit=$?` o amb PYTHONIOENCODING=utf-8. Al
// banc era un pas sencer de més en 9 de 32 tirades (docs/research/speed.md).
// Només PYTHONIOENCODING (stdin, stdout i stderr): PYTHONUTF8=1 canviaria
// també la codificació per defecte d'open(), i el codi que l'agent prova
// es comportaria diferent del que l'usuari executa fora de Gregal.
var utf8Env = []string{"PYTHONIOENCODING=utf-8"}

// ChildEnv és l'entorn de les ordres de l'agent (bash i bash_background).
// Fora de Windows torna nil (exec hereta l'entorn tal qual); a Windows
// hi afegeix utf8Env, sense trepitjar el que l'usuari ja hagi definit.
func ChildEnv() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return ambUTF8(os.Environ())
}

func ambUTF8(env []string) []string {
	out := env
	for _, kv := range utf8Env {
		nom := kv[:strings.IndexByte(kv, '=')+1]
		definit := false
		for _, e := range env {
			if len(e) >= len(nom) && strings.EqualFold(e[:len(nom)], nom) {
				definit = true
				break
			}
		}
		if !definit {
			out = append(out, kv)
		}
	}
	return out
}

// DecodeOutput torna la sortida d'una ordre com a text UTF-8. Si no és
// UTF-8 vàlid a Windows, es llegeix com a cp1252 (la pàgina ANSI dels
// programes natius que no en saben res d'UTF-8): una «É» de veritat diu
// més al model que un «�» que el convida a repetir l'ordre.
func DecodeOutput(b []byte) string {
	if runtime.GOOS != "windows" || utf8.Valid(b) {
		return string(b)
	}
	return decodeCP1252(b)
}

func decodeCP1252(b []byte) string {
	s, err := charmap.Windows1252.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(s)
}
