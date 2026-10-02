package agent

import (
	"regexp"
	"strings"
)

// Crides d'eina escrites com a text.
//
// Alguns models —els de la família Qwen i els entrenats amb el format de
// Hermes, que són molts dels que es fan servir en local— quan la crida
// estructurada no els surt, l'escriuen al cos del missatge:
//
//	<tool_call><function=write><parameter=path>…</parameter></function></tool_call>
//
// El client no hi veu cap tool_call, o sigui que per a l'agent el torn
// s'ha acabat, i això anava directament a la pantalla. Vist amb el model
// real: l'usuari acabava llegint XML en comptes de la feina feta, sense
// cap indici que el que havia passat és que el model havia intentat fer
// una cosa i no havia pogut.
//
// Aquí no s'executa res del que hi digui: ve del model, no de l'usuari, i
// executar una crida que el nostre propi client no ha reconegut com a tal
// seria saltar-se la validació i els permisos. Només es treu del text i es
// diu què ha passat.

var reEinaText = regexp.MustCompile(`(?is)<tool_call>.*?(</tool_call>|$)|<function\s*=.*?(</function>|$)`)

// SenseEinaText treu del text les crides d'eina escrites en cru i diu si
// n'hi havia cap.
func SenseEinaText(s string) (net string, hiEra bool) {
	if !reEinaText.MatchString(s) {
		return s, false
	}
	net = strings.TrimSpace(reEinaText.ReplaceAllString(s, ""))
	// Sovint queda la frase d'abans («Ho refaig amb el PATH…»), que sí que
	// val la pena ensenyar. Si no queda res, el text era tot la crida.
	return net, true
}

// AvisEinaText és el que se li diu a qui mira la pantalla quan passa.
const AvisEinaText = "El model ha intentat cridar una eina escrivint-la al text en comptes de cridar-la, i això no s'executa. Torna-ho a demanar; si passa sovint, aquest model no se'n surt amb les eines i val més canviar-lo al rol code."

// GuiaEinaText és el que se li diu al MODEL quan passa: no s'ha executat
// res, però en comptes de tancar el torn se li demana que refaci la crida
// amb tool_calls estructurats (acotat: vegeu textRetries als fronts).
const GuiaEinaText = "La teva crida d'eina escrita al text NO s'ha executat: les eines només funcionen via tool_calls estructurats. Torna a fer la mateixa crida amb tool_calls, sense escriure-la al text."
