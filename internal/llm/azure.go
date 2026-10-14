package llm

import (
	"net/http"
	"net/url"
	"strings"
)

// Azure OpenAI i Microsoft Foundry.
//
// Gregal hi parla com amb qualsevol endpoint OpenAI, però tres detalls
// canvien (documentació de Microsoft, consultada el 2026-10-14):
//
//   - Clau d'API: la REST d'Azure la vol a la capçalera `api-key`.
//     `Authorization: Bearer` és per als tokens d'Entra ID (JWT).
//   - URLs clàssiques: /openai/deployments/<desplegament>?api-version=...
//     porten la consulta al final; el camí (/chat/completions) va abans del
//     «?». La recomanada és https://<recurs>.openai.azure.com/openai/v1/
//     (o .services.ai.azure.com/openai/v1/), sense api-version i amb el
//     nom del desplegament com a model.
//   - API Model Inference de Foundry (/models, *.models.ai.azure.com,
//     *.inference.ai.azure.com): qualsevol paràmetre fora del seu estàndard
//     (reasoning_effort, per exemple) és un error si no s'envia
//     `extra-parameters: pass-through`, que el fa arribar al model.

var azureHosts = []string{
	".openai.azure.com", ".services.ai.azure.com", ".cognitiveservices.azure.com",
	".models.ai.azure.com", ".inference.ai.azure.com",
}

func hostOf(baseURL string) (string, string) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", ""
	}
	return strings.ToLower(u.Hostname()), strings.ToLower(u.Path)
}

// isAzure diu si l'endpoint és d'Azure OpenAI o de Foundry.
func isAzure(baseURL string) bool {
	h, _ := hostOf(baseURL)
	for _, s := range azureHosts {
		if strings.HasSuffix(h, s) {
			return true
		}
	}
	return false
}

// isFoundryInference diu si és l'API Model Inference de Foundry (no la
// d'Azure OpenAI), la que demana extra-parameters per a camps no estàndard.
func isFoundryInference(baseURL string) bool {
	h, p := hostOf(baseURL)
	if strings.HasSuffix(h, ".models.ai.azure.com") || strings.HasSuffix(h, ".inference.ai.azure.com") {
		return true
	}
	return strings.HasSuffix(h, ".services.ai.azure.com") && (p == "/models" || strings.HasPrefix(p, "/models/"))
}

// endpointURL afegeix el camí (p. ex. /chat/completions) a la base,
// respectant una consulta (?api-version=...) si n'hi ha.
func endpointURL(baseURL, path string) string {
	base := strings.TrimSpace(baseURL)
	query := ""
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base, query = base[:i], base[i:]
	}
	return strings.TrimSuffix(base, "/") + path + query
}

// setAuth posa la credencial: a Azure, una clau d'API va a `api-key` i un
// token d'Entra ID (JWT, comença per «eyJ») a Bearer; a la resta, Bearer.
// A la Model Inference de Foundry també demana passar els camps extra.
func setAuth(req *http.Request, baseURL, apiKey string) {
	key := strings.TrimSpace(apiKey)
	azure := isAzure(baseURL)
	switch {
	case key == "":
	case azure && !strings.HasPrefix(key, "eyJ"):
		req.Header.Set("api-key", key)
	default:
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if azure && isFoundryInference(baseURL) {
		req.Header.Set("extra-parameters", "pass-through")
	}
}
