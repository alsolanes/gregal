package agent

// Via ràpida de cortesia: un "hola" ha de tornar un "hola", no un torn
// d'agent amb glob/bash que crema passos i diners. Abans, qualsevol text
// curt passava pel model amb totes les eines i un raonador avorrit es
// posava a explorar el repo per respondre una salutació.
//
// SmallTalk retorna (resposta, cert) si el text és pura cortesia
// (salutació, gràcies, comiat o assentiment curt, ≤28 caràcters, sense
// salts de línia): cap crida al model, cap eina, zero cost. "hola,
// resumeix el document" NO hi entra (té feina a fer) i va a l'agent.

import (
	"strings"
)

type smallKind int

const (
	kindGreet smallKind = iota
	kindThanks
	kindBye
	kindAck
)

type smallHit struct {
	kind smallKind
	lang string // "ca", "es" o "en"
}

var smallTalkPhrases = map[string]smallHit{
	// Català.
	"hola": {kindGreet, "ca"}, "bones": {kindGreet, "ca"}, "ei": {kindGreet, "ca"},
	"salut": {kindGreet, "ca"}, "bon dia": {kindGreet, "ca"},
	"bona tarda": {kindGreet, "ca"}, "bon vespre": {kindGreet, "ca"},
	"bona nit": {kindGreet, "ca"}, "què tal": {kindGreet, "ca"},
	"que tal": {kindGreet, "ca"}, "com va": {kindGreet, "ca"},
	"com anem": {kindGreet, "ca"},
	"gràcies":  {kindThanks, "ca"}, "gracies": {kindThanks, "ca"},
	"moltes gràcies": {kindThanks, "ca"}, "moltes gracies": {kindThanks, "ca"},
	"merci": {kindThanks, "ca"},
	"adeu":  {kindBye, "ca"}, "adéu": {kindBye, "ca"},
	"fins ara": {kindBye, "ca"}, "fins després": {kindBye, "ca"},
	"fins aviat": {kindBye, "ca"},
	"ok":         {kindAck, "ca"}, "okey": {kindAck, "ca"}, "vale": {kindAck, "ca"},
	"d'acord": {kindAck, "ca"}, "dacord": {kindAck, "ca"},
	"perfecte": {kindAck, "ca"}, "genial": {kindAck, "ca"},
	"entès": {kindAck, "ca"}, "entes": {kindAck, "ca"},
	// Castellà.
	"buenas": {kindGreet, "es"}, "oye": {kindGreet, "es"},
	"saludos": {kindGreet, "es"}, "buenos dias": {kindGreet, "es"},
	"buenos días": {kindGreet, "es"}, "buenas tardes": {kindGreet, "es"},
	"buenas noches": {kindGreet, "es"}, "qué tal": {kindGreet, "es"},
	"cómo va": {kindGreet, "es"}, "como va": {kindGreet, "es"}, "ey": {kindGreet, "es"},
	"gracias": {kindThanks, "es"}, "muchas gracias": {kindThanks, "es"},
	"adios": {kindBye, "es"}, "adiós": {kindBye, "es"},
	"hasta luego": {kindBye, "es"}, "hasta pronto": {kindBye, "es"},
	"nos vemos": {kindBye, "es"},
	"perfecto":  {kindAck, "es"},
	// Anglès.
	"hi": {kindGreet, "en"}, "hello": {kindGreet, "en"},
	"hey": {kindGreet, "en"}, "yo": {kindGreet, "en"},
	"howdy": {kindGreet, "en"}, "good morning": {kindGreet, "en"},
	"good afternoon": {kindGreet, "en"}, "good evening": {kindGreet, "en"},
	"good night": {kindGreet, "en"}, "what's up": {kindGreet, "en"},
	"whats up": {kindGreet, "en"}, "sup": {kindGreet, "en"},
	"thanks": {kindThanks, "en"}, "thank you": {kindThanks, "en"},
	"thx": {kindThanks, "en"}, "ty": {kindThanks, "en"},
	"bye": {kindBye, "en"}, "goodbye": {kindBye, "en"},
	"see you": {kindBye, "en"}, "see ya": {kindBye, "en"}, "cya": {kindBye, "en"},
	"okay": {kindAck, "en"}, "perfect": {kindAck, "en"},
}

var smallTalkReplies = map[smallKind]map[string]string{
	kindGreet: {
		"ca": "Hola! Sóc el Gregal: puc llegir i editar el projecte, executar ordres i respondre preguntes. Què fem?",
		"es": "¡Hola! Soy Gregal: puedo leer y editar el proyecto, ejecutar comandos y responder preguntas. ¿Qué hacemos?",
		"en": "Hi! I'm Gregal: I can read and edit the project, run commands and answer questions. What shall we do?",
	},
	kindThanks: {
		"ca": "De res! Aquí estic per al que calgui.",
		"es": "¡De nada! Aquí estoy para lo que necesites.",
		"en": "You're welcome! Here whenever you need me.",
	},
	kindBye: {
		"ca": "Fins ara! Ja saps on sóc.",
		"es": "¡Hasta luego!",
		"en": "See you!",
	},
	kindAck: {
		"ca": "Perfecte, digues-m'hi.",
		"es": "Perfecto, dime.",
		"en": "Got it, go ahead.",
	},
}

// SmallTalk diu si el text és pura cortesia i, si ho és, amb què respondre.
// Normalitza (minúscules, espais, signes): "  Hola! " també és un hola.
func SmallTalk(text string) (string, bool) {
	norm := strings.ToLower(strings.TrimSpace(text))
	norm = strings.Trim(norm, "?!.,;¡¿·…\"'()")
	norm = strings.Join(strings.Fields(norm), " ")
	if norm == "" || len([]rune(norm)) > 28 || strings.Contains(norm, "\n") {
		return "", false
	}
	hit, ok := smallTalkPhrases[norm]
	if !ok {
		return "", false
	}
	return smallTalkReplies[hit.kind][hit.lang], true
}
