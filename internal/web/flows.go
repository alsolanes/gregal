package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"gregal/internal/flow"
	"gregal/internal/llm"
)

// API dels grafs. Els grafs viuen al projecte de la sessió (.gregal/flows),
// no a la carpeta de l'usuari, i per tant totes aquestes rutes van per
// sessió: si canvies de workspace, canvien els grafs.

func (s *Server) handleFlows(w http.ResponseWriter, r *http.Request) {
	llista, err := flow.Llista(s.cwd)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"flows": llista, "dir": flow.DirFor(s.cwd)})
}

func (s *Server) handleFlowLoad(w http.ResponseWriter, r *http.Request) {
	nom := r.URL.Query().Get("name")
	if strings.TrimSpace(nom) == "" {
		http.Error(w, "cal un nom", 400)
		return
	}
	f, err := flow.Carrega(s.cwd, nom)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, map[string]any{"flow": f})
}

func (s *Server) handleFlowSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Flow *flow.Flow `json:"flow"`
		// Rename permet desar amb un nom nou i esborrar el vell d'una
		// tacada; si no, canviar el nom deixaria el graf antic al disc.
		Rename string `json:"rename,omitempty"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Flow == nil {
		http.Error(w, "petició il·legible", 400)
		return
	}
	path, err := flow.Desa(s.cwd, req.Flow)
	if err != nil {
		// Un graf mal fet és error de l'usuari, no del servidor: 400 perquè
		// la UI el pugui ensenyar al costat del dibuix.
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Rename != "" && flow.Slug(req.Rename) != flow.Slug(req.Flow.Name) {
		_ = flow.Esborra(s.cwd, req.Rename)
	}
	writeJSON(w, map[string]any{"ok": true, "path": path, "slug": flow.Slug(req.Flow.Name)})
}

func (s *Server) handleFlowDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "cal un nom", 400)
		return
	}
	if err := flow.Esborra(s.cwd, req.Name); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// handleFlowRun executa un graf i va enviant els passos per SSE: "start"
// en començar cada pas, "step" en acabar-lo, "done" al final.
//
// És GET perquè al davant hi ha un EventSource. Tancar l'EventSource
// cancel·la el context i atura el graf on sigui: no cal cap ruta per
// aturar-lo.
func (s *Server) handleFlowRun(w http.ResponseWriter, r *http.Request) {
	nom := r.URL.Query().Get("name")
	f, err := flow.Carrega(s.cwd, nom)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	// Un sol treballador: si l'agent interactiu o un job estan en marxa, un
	// graf hi entraria pel mig i es barrejarien els canvis al disc.
	s.mu.Lock()
	if s.agentBusy || s.jobRunning || s.flowRunning {
		s.mu.Unlock()
		http.Error(w, "l'agent està ocupat: prova-ho d'aquí un moment", 409)
		return
	}
	s.flowRunning = true
	permissiu := s.permissive
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.flowRunning = false
		s.mu.Unlock()
	}()

	// Un graf corre sol i ningú mirarà el diàleg de permisos. auto=1 diu
	// que les eines «ask» passin; les «deny» continuen bloquejades sempre.
	auto := permissiu || r.URL.Query().Get("auto") == "1"

	emit := sse(w)
	runner := &flow.AgentRunner{
		Cfg: s.cfg, Client: s.client, Policy: s.policy,
		Mode: s.mode, AutoApprove: auto,
	}
	estat := flow.State{}
	if v := r.URL.Query().Get("input"); v != "" {
		estat["input"] = v
	}

	emit("start", map[string]any{"flow": f.Name, "steps": len(f.Nodes), "auto": auto})
	ctx := r.Context()
	res, runErr := flow.Run(ctx, f, runner, flow.Opcions{
		Estat: estat,
		OnStep: func(st flow.StepResult) {
			emit("step", map[string]any{
				"node": st.Node, "kind": st.Kind, "title": st.Title,
				"input": retalla(st.Input, 4000), "output": retalla(st.Output, 20000),
				"error": st.Err, "ms": st.Took.Milliseconds(),
			})
		},
	})
	fi := map[string]any{"steps": len(res.Steps), "stopped": res.Stopped}
	if runErr != nil && ctx.Err() == nil {
		fi["error"] = runErr.Error()
	}
	if last, ok := res.Last(); ok {
		fi["answer"] = last.Output
	}
	emit("done", fi)

	// El resultat queda a la conversa perquè després en puguis parlar amb
	// l'agent: un graf que s'executa i desapareix no serveix de gaire.
	s.recordFlowRun(f, res, time.Now())
}

// recordFlowRun deixa a la conversa un resum del que ha fet el graf.
func (s *Server) recordFlowRun(f *flow.Flow, res flow.RunResult, quan time.Time) {
	var b strings.Builder
	b.WriteString("S'ha executat el graf «" + f.Name + "»:\n\n")
	for _, st := range res.Steps {
		estat := "ok"
		if st.Err != "" {
			estat = "error: " + st.Err
		}
		b.WriteString("- " + etiquetaPas(st) + " (" + estat + ")\n")
	}
	if res.Stopped != "" {
		b.WriteString("\nAturat: " + res.Stopped + "\n")
	}
	if last, ok := res.Last(); ok && strings.TrimSpace(last.Output) != "" {
		b.WriteString("\nÚltima sortida:\n" + retalla(last.Output, 4000))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.convo = append(s.convo,
		llm.Message{Role: "user", Content: "Executa el graf «" + f.Name + "»."},
		llm.Message{Role: "assistant", Content: b.String()})
	s.autosaveLocked()
}

func etiquetaPas(st flow.StepResult) string {
	if strings.TrimSpace(st.Title) != "" {
		return st.Title
	}
	return st.Node
}

// retalla escapça un text llarg. La sortida d'un pas pot ser un build
// sencer i no cal que viatgi tot al navegador.
func retalla(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(retallat)"
}
