package web

import "testing"

func TestFriendlyRunError(t *testing.T) {
	casos := []struct {
		in, vol string
	}{
		// DNS: no es resol el domini del proveïdor (el cas real de la captura del mòbil).
		{`provider https://opencode.ai/zen/go/v1: Post "https://opencode.ai/zen/go/v1/chat/completions": dial tcp: lookup opencode.ai: no such host (host not found: check the provider base_url)`,
			"No s'ha pogut arribar al proveïdor: el dispositiu no resol l'adreça opencode.ai. Comprova que hi ha internet i torna-ho a provar."},
		{`dial tcp: lookup api.deepseek.com: no such host`,
			"No s'ha pogut arribar al proveïdor: el dispositiu no resol l'adreça api.deepseek.com. Comprova que hi ha internet i torna-ho a provar."},
		// Connexió refusada / timeout: el servidor del proveïdor no respon.
		{`provider http://x: Post "http://x": dial tcp: 1.2.3.4:443: connect: connection refused`,
			"No s'ha pogut arribar al proveïdor (connexió refusada). Pot ser un tall puntual: torna-ho a provar més tard."},
		{`context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
			"El proveïdor ha trigat massa a respondre. Torna-ho a provar: si continua així, pot estar saturat."},
		// Certificat.
		{`tls: failed to verify certificate: x509: certificate signed by unknown authority`,
			"El proveïdor presenta un certificat que no es confia. Comprova la data/hora del dispositiu."},
		// Error del model amb codi HTTP.
		{`provider http://x: 401 Unauthorized`,
			"El proveïdor ha rebutjat la clau (401). Revisa la clau API a la configuració."},
		{`provider http://x: 429 Too Many Requests`,
			"El proveïdor ha limitat les peticions (429). Espera una mica i torna-ho a provar."},
		// Sense connexió genèrica.
		{`no route to host`,
			"No hi ha ruta cap al servidor. Comprova que hi ha internet i torna-ho a provar."},
		{`EOF`,
			"La connexió amb el proveïdor s'ha tallat a mig camí. Torna-ho a provar."},
		// Res conegut: el text original, tal com és (mai amagar informació).
		{"el model ha dit que no", "el model ha dit que no"},
	}
	for _, c := range casos {
		if got := friendlyRunError(c.in); got != c.vol {
			t.Errorf("friendlyRunError(%q)\n vol : %s\n got : %s", c.in, c.vol, got)
		}
	}
}
