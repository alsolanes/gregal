package web

import "strings"

// friendlyRunError tradueix un error de connexió del proveïdor (text cru del Go,
// com el que surt al detall d'una execució) a una frase que una persona entén:
// què ha passat i què fer-hi. Res conegut es deixa tal qual — mai amagar
// informació. La mateixa taula existeix a l'app i a la web.
func friendlyRunError(raw string) string {
	e := strings.TrimSpace(raw)
	if e == "" {
		return e
	}
	l := strings.ToLower(e)
	host := hostDeError(e)

	switch {
	case strings.Contains(l, "no such host"), strings.Contains(l, "dns"):
		if host != "" {
			return "No s'ha pogut arribar al proveïdor: el dispositiu no resol l'adreça " + host + ". Comprova que hi ha internet i torna-ho a provar."
		}
		return "No s'ha pogut arribar al proveïdor: no es resol la seva adreça. Comprova que hi ha internet i torna-ho a provar."
	case strings.Contains(l, "connection refused"):
		return "No s'ha pogut arribar al proveïdor (connexió refusada). Pot ser un tall puntual: torna-ho a provar més tard."
	case strings.Contains(l, "context deadline exceeded"), strings.Contains(l, "client.timeout"), strings.Contains(l, "timeout"):
		return "El proveïdor ha trigat massa a respondre. Torna-ho a provar: si continua així, pot estar saturat."
	case strings.Contains(l, "certificate"), strings.Contains(l, "x509"):
		return "El proveïdor presenta un certificat que no es confia. Comprova la data/hora del dispositiu."
	case strings.Contains(l, "401"), strings.Contains(l, "unauthorized"):
		return "El proveïdor ha rebutjat la clau (401). Revisa la clau API a la configuració."
	case strings.Contains(l, "429"), strings.Contains(l, "too many requests"):
		return "El proveïdor ha limitat les peticions (429). Espera una mica i torna-ho a provar."
	case strings.Contains(l, "no route to host"), strings.Contains(l, "network is unreachable"):
		return "No hi ha ruta cap al servidor. Comprova que hi ha internet i torna-ho a provar."
	case l == "eof", strings.HasSuffix(l, "eof"):
		return "La connexió amb el proveïdor s'ha tallat a mig camí. Torna-ho a provar."
	}
	return e
}

// hostDeError extreu l'host de l'error del transport ("dial tcp: lookup HOST:")
// per poder-lo mostrar. Cadena buida si no hi és.
func hostDeError(e string) string {
	if i := strings.Index(e, "lookup "); i >= 0 {
		rest := e[i+len("lookup "):]
		if j := strings.IndexAny(rest, ": "); j > 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}
