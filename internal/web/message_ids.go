package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"gregal/internal/llm"
)

// Identificadors estables de missatge (contracte de l'assistent, B0).
//
// Un missatge nou rep un id «m-…» en néixer. Els missatges antics no en
// tenen: se'ls en deriva un de determinista «h-…» a partir del rol, l'hora i
// el text, de manera que el mateix missatge té el mateix id a totes les rutes
// i entre reinicis sense cap migració de dades.

// newMessageID crea l'id d'un missatge nou: ordenable per temps i únic.
func newMessageID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return "m-" + strconv.FormatInt(time.Now().UnixNano(), 36) + hex.EncodeToString(b[:])
}

// messageID torna l'id del missatge: el desat o, si és antic, el derivat.
func messageID(m llm.Message) string {
	if m.ID != "" {
		return m.ID
	}
	h := sha256.Sum256([]byte(m.Role + "\x00" + strconv.FormatInt(m.Ts, 10) + "\x00" + m.Content))
	return "h-" + hex.EncodeToString(h[:8])
}
