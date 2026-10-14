package agent

import "gregal/internal/llm"

// SenseMapesAntics torna una còpia de l'historial sense el mapa del
// projecte (MarcaContext, i els fitxers adjunts que hi van) als missatges
// d'usuari. Els clients la fan servir abans d'afegir la tasca nova amb el
// seu mapa: així només el missatge més nou porta un mapa, i és el vigent.
// Abans cada torn hi deixava el seu (amb un estat de git i uns fitxers que
// ja no eren veritat) i la conversa creixia milers de tokens per torn.
// L'historial original no es modifica.
func SenseMapesAntics(hist []llm.Message) []llm.Message {
	out := make([]llm.Message, len(hist))
	copy(out, hist)
	for i := range out {
		if out[i].Role == "user" {
			out[i].Content = SenseMapa(out[i].Content)
		}
	}
	return out
}

// TeConversa diu si l'historial ja té alguna resposta de l'assistent: a
// partir d'aquí, un «ok» és la resposta a una pregunta (SmallTalkFor).
func TeConversa(hist []llm.Message) bool {
	for _, m := range hist {
		if m.Role == "assistant" {
			return true
		}
	}
	return false
}
