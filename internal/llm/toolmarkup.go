package llm

import "strings"

// SenseCridesText treu el markup DSML de crides d'eina d'un text que ha de
// ser una resposta i diu si n'hi havia. Les crides sense tools (la síntesi
// final, la pregunta d'ampliació) no passen per parseDSML com a crides:
// quan el model hi vol cridar una eina igualment, el markup arribava a
// l'usuari com a resposta (vist amb deepseek-v4.1-flash en tallar per
// --max-steps just després d'escriure fitxers).
func SenseCridesText(s string) (string, bool) {
	if !hasDSML(s) {
		return s, false
	}
	// Els blocs sencers fora; d'un bloc tallat (sense tancament), tot el que
	// ve després de la primera etiqueta és markup.
	net := dsmlBlock.ReplaceAllString(s, "")
	if loc := dsmlAny.FindStringIndex(net); loc != nil {
		net = net[:loc[0]]
	}
	return strings.TrimSpace(net), true
}
