package web

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

// Logs del servidor: cicle de vida de torns i equips al terminal (stderr)
// i, opcionalment, a un fitxer. Volum baix per disseny (unes línies per
// torn): sense rotació.
//
// Regla d'or: MAI credencials als logs. Els errors de proveïdor poden dur
// URLs amb claus, així que tot text lliure passa per redact().

var (
	reLogURL    = regexp.MustCompile(`https?://[^\s"'<>]+`)
	reLogSecret = regexp.MustCompile(`(?i)\b(api[_-]?key|token|secret|password|passwd|client[_-]?secret)\b(\s*[:=]\s*["']?)[^\s"'<>,}\]]+`)
	reLogSk     = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`)
	reLogBearer = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-._~+/=]+`)
)

// redact treu URLs senceres i valors de secrets d'un text lliure. Davant
// del dubte, esborra de més: un log sense una dada val més que una clau
// filtrada.
func redact(s string) string {
	s = reLogURL.ReplaceAllString(s, "[url]")
	s = reLogSecret.ReplaceAllString(s, "$1=[redacted]")
	s = reLogSk.ReplaceAllString(s, "[key]")
	s = reLogBearer.ReplaceAllString(s, "Bearer [redacted]")
	return s
}

func redactError(err error) string {
	if err == nil {
		return ""
	}
	return redact(err.Error())
}

// previewTask resumeix la tasca a la primera línia, retallada i sanejada.
func previewTask(task string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(task), "\n")
	return redact(workshopText(line, 120))
}

// turnOutcome classifica el final d'un torn per al log: fet, aturat per
// l'usuari, o fallit amb l'error sanejador.
func turnOutcome(err error) string {
	if err == nil {
		return "done"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "failed: " + redactError(err)
}

// logTurnStarted deixa constància de l'inici amb el temps; el final el
// pinta qui tanca el torn amb turnOutcome.
func logTurnStarted(session, mode, task string) time.Time {
	log.Printf("turn started session=%s mode=%s task=%s", session, mode, previewTask(task))
	return time.Now()
}

// SetupServerLog duplica el log estàndard al terminal i, si path no és
// buit, també a un fitxer en mode append amb permisos 0600. Torna una
// neteja per tancar el fitxer.
func SetupServerLog(path string) (func(), error) {
	if strings.TrimSpace(path) == "" {
		return func() {}, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return func() { _ = f.Close() }, nil
}
