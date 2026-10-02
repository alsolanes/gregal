package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// convoLlarga fabrica una conversa que segur que passa del llindar.
func convoLlarga(missatges int) []llm.Message {
	gruix := strings.Repeat("línia de feina amb prou text per omplir context. ", 200)
	out := make([]llm.Message, 0, missatges)
	for i := 0; i < missatges; i++ {
		rol := "user"
		if i%2 == 1 {
			rol = "assistant"
		}
		out = append(out, llm.Message{Role: rol, Content: gruix})
	}
	return out
}

func servidorAmbResumidor(t *testing.T, resum string, trucades *atomic.Int32) *Server {
	t.Helper()
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trucades.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"` + resum + `"}}]}`))
	}))
	t.Cleanup(prov.Close)
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: prov.URL + "/v1"}}
	return s
}

// El TUI compactava sol des del principi; la finestra, que és on es passen
// les hores, no compactava mai. Una sessió llarga creixia fins que el model
// retallava el context pel seu compte —sense dir-ho— o petava.
func TestLaFinestraCompactaSolaQuanLaConversaNoHiCap(t *testing.T) {
	var trucades atomic.Int32
	s := servidorAmbResumidor(t, "Resum dens del que portàvem.", &trucades)
	s.convo = convoLlarga(40)
	p, role := s.roleRef()

	s.compactaSiCal(context.Background(), p, role, nil)

	// Amb una conversa tan llarga, el resum és jeràrquic: diverses crides
	// (map) i una de combinació (reduce). El que importa és que es faci.
	if trucades.Load() < 1 {
		t.Fatalf("calia demanar un resum al model, i no s'ha fet cap crida")
	}
	if len(s.convo) != 4 {
		t.Errorf("després de compactar han de quedar els 4 darrers missatges, en queden %d", len(s.convo))
	}
	if s.compacted == "" {
		t.Error("el resum no s'ha desat")
	}
	if !strings.Contains(s.sysPrompt(), "Resum dens del que portàvem.") {
		t.Error("el resum no arriba al system prompt: el torn següent oblidaria tot el que s'ha fet")
	}
}

// Una conversa que hi cap no s'ha de tocar: compactar quan no cal costa una
// crida al model i perd el detall dels missatges recents.
func TestNoCompactaSiLaConversaHiCap(t *testing.T) {
	var trucades atomic.Int32
	s := servidorAmbResumidor(t, "no caldria", &trucades)
	s.convo = []llm.Message{
		{Role: "user", Content: "hola"},
		{Role: "assistant", Content: "hola"},
		{Role: "user", Content: "què tal"},
		{Role: "assistant", Content: "bé"},
		{Role: "user", Content: "gràcies"},
	}
	p, role := s.roleRef()
	s.compactaSiCal(context.Background(), p, role, nil)
	if trucades.Load() != 0 {
		t.Errorf("no calia resumir res i s'han fet %d crides", trucades.Load())
	}
	if len(s.convo) != 5 || s.compacted != "" {
		t.Errorf("la conversa s'ha tocat sense caldre: %d missatges, resum %q", len(s.convo), s.compacted)
	}
}

// Si el resum falla, val més continuar amb la conversa llarga que perdre el
// torn: el model encara pot respondre, i si no, l'error serà seu i visible.
func TestSiElResumFallaElTornContinua(t *testing.T) {
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer prov.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: prov.URL + "/v1"}}
	s.convo = convoLlarga(40)
	p, role := s.roleRef()

	s.compactaSiCal(context.Background(), p, role, nil)

	if len(s.convo) != 40 {
		t.Errorf("amb el resum fallat la conversa s'ha de quedar sencera, i en queden %d", len(s.convo))
	}
	if s.compacted != "" {
		t.Errorf("s'ha desat un resum que no existeix: %q", s.compacted)
	}
}

// Començar de nou ha d'oblidar també el resum: si no, el system prompt de la
// conversa nova porta el context de la feina anterior.
func TestLaConversaNovaOblidaElResum(t *testing.T) {
	var trucades atomic.Int32
	s := servidorAmbResumidor(t, "feina vella", &trucades)
	s.convo = convoLlarga(6)
	s.compacted = "feina vella"
	rec := httptest.NewRecorder()
	s.handleNew(rec, httptest.NewRequest(http.MethodPost, "/api/new", nil))
	if s.compacted != "" {
		t.Errorf("el resum ha sobreviscut a la conversa nova: %q", s.compacted)
	}
	if strings.Contains(s.sysPrompt(), "feina vella") {
		t.Error("el system prompt encara porta el resum de l'altra conversa")
	}
}

// Alguns models, quan la crida estructurada no els surt, l'escriuen al cos
// del missatge. Per a l'agent el torn s'ha acabat, i l'XML anava tal qual
// a la pantalla: l'usuari llegia markup en comptes de la feina feta.
func TestLaCridaEscritaAlTextNoArribaALaPantalla(t *testing.T) {
	cru := `{"choices":[{"message":{"content":"Ara ho escric.\n<tool_call>\n<function=write>\n<parameter=path>\n/tmp/a\n</parameter>\n</function>\n</tool_call>"}}]}`
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cru))
	}))
	defer prov.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: prov.URL + "/v1"}}
	rec := httptest.NewRecorder()
	s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"fes-ho"}`)))
	cos := rec.Body.String()
	// El que compta és el que queda a la pantalla. Els tokens del carrer
	// arriben tal com raja —és el que el model escriu— i la bombolla viva
	// se substitueix per la resposta final; el que no pot portar l'XML és
	// aquesta, que és la que es desa a la conversa.
	final := cos[strings.LastIndex(cos, "event: done"):]
	if strings.Contains(final, "tool_call") || strings.Contains(final, "function=write") {
		t.Errorf("la resposta final porta l'XML de la crida:\n%s", final)
	}
	if !strings.Contains(cos, "tool_text") {
		t.Errorf("no s'ha avisat que el model ho ha intentat i no ha pogut:\n%s", cos)
	}
	if !strings.Contains(cos, "Ara ho escric") {
		t.Errorf("s'ha menjat el text de debò:\n%s", cos)
	}
}
