package web

import (
	"fmt"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// keylessHint: en una instal·lació nova els rols apunten a un proveïdor
// remot sense clau, i el primer missatge acabava amb el 401 en JSON del
// proveïdor («You didn't provide an API key…»). Si l'error és d'autenticació
// es diu què passa i on es resol (la web obre el mateix diàleg de primer
// inici des de Preferències): sense clau, que cal posar-n'hi una; amb clau,
// que el proveïdor no l'accepta. Qualsevol altre error es retorna tal qual:
// res no s'amaga.
func (s *Server) keylessHint(p config.Provider, name, raw string) string {
	// Amb clau, només un rebuig clar (401/403) diu que la clau no va: un
	// «api key» solt podria ser d'una eina, no del proveïdor.
	if p.BaseURL == "" || llm.IsLocalBase(p.BaseURL) || !authFailure(raw, p.HasAPIKey()) {
		return raw
	}
	if name == "" {
		name = p.BaseURL
	}
	if p.HasAPIKey() {
		return fmt.Sprintf(s.tr(
			"«%s» no accepta la clau d'API (pot ser incompleta, caducada o sense crèdit). Revisa-la a Preferències → Models i agent (Ctrl+,).",
			"“%s” does not accept the API key (it may be incomplete, expired or out of credit). Check it in Preferences → Models & agent (Ctrl+,)."), name)
	}
	return fmt.Sprintf(s.tr(
		"Encara no hi ha cap clau d'API per a «%s», i sense clau el model no pot respondre. Connecta un model a Preferències → Models i agent (Ctrl+,).",
		"There is no API key for “%s” yet, so the model cannot answer. Connect a model in Preferences → Models & agent (Ctrl+,)."), name)
}

// authFailure diu si el text d'error és d'un proveïdor que no ha acceptat
// (o no ha rebut) la clau. strict demana un codi o un tipus d'error
// d'autenticació, no només que s'hi parli de claus.
func authFailure(raw string, strict bool) bool {
	l := strings.ToLower(raw)
	keys := []string{"http 401", "http 403", "401 unauthorized", "unauthorized", "invalid_api_key", "authentication_error"}
	if !strict {
		keys = append(keys, "api key", "api_key", "x-api-key")
	}
	for _, k := range keys {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}
