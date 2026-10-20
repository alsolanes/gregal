package web

import (
	"context"
	"regexp"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// El cervell del vent dins el servidor: sap on viu l'estat de l'usuari i
// fa la crida petita d'extracció de fets després de cada resposta.

// windStateDir és el directori d'estat dels vents d'aquesta sessió: el de
// l'usuari si n'hi ha, o el compartit del config (mode local).
func (s *Server) windStateDir() string {
	if s.user != nil && !s.user.Tothom {
		return userStateDir(s.cfgPath, s.user.Name)
	}
	return s.goalDir
}

// windBrain torna el que el vent ja sap ("" si encara res).
func (s *Server) windBrain(windID string) string {
	return companion.LoadWindBrain(s.windStateDir(), windID)
}

// windLearn extreu els fets duradors d'una resposta i els desa al cervell.
// Es fa en segon pla: una crida petita amb els fets ja coneguts per no
// repetir-se, i res no ha de fallar la conversa si això peta.
func (s *Server) startWindLearn(w companion.Wind, question, answer string) {
	// Capture before spawning: even an extraction not yet scheduled must not
	// repopulate knowledge that the user clears immediately after the reply.
	revision := companion.WindBrainRevision(s.windStateDir(), w.ID)
	go s.windLearnRevision(w, question, answer, revision)
}

func (s *Server) windLearn(w companion.Wind, question, answer string) {
	revision := companion.WindBrainRevision(s.windStateDir(), w.ID)
	s.windLearnRevision(w, question, answer, revision)
}

func (s *Server) windLearnRevision(w companion.Wind, question, answer string, revision uint64) {
	defer func() { _ = recover() }() // el cervell no pot tombar el xat
	p, role, ok := s.sideRoleRef()
	if !ok {
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	known := companion.WindBrainNotes(s.windStateDir(), w.ID)
	var b strings.Builder
	b.WriteString("You maintain the knowledge base of " + w.Name + ", a companion that watches: " + w.Role + ".\n")
	b.WriteString("From the exchange below, extract DURABLE facts about the topic (specs, prices, release dates, definitions — not chit-chat).\n")
	b.WriteString("Reply ONLY with bullet lines starting with '- ', one fact per line, max 4 lines, in the user's language. Each fact must carry its source domain in brackets like [notebookcheck.com].\n")
	b.WriteString("If nothing is durable, reply with nothing.\n")
	if len(known) > 0 {
		b.WriteString("\nAlready known (do NOT repeat these):\n")
		for i, k := range known {
			if i >= 30 {
				break
			}
			b.WriteString("- " + k + "\n")
		}
	}
	b.WriteString("\nExchange:\nUser: " + question + "\n" + w.Name + ": " + answer)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := windLearnLoop(s, ctx, client, cfg, p, role, []llm.Message{{Role: "user", Content: b.String()}})
	if err != nil {
		return
	}
	// Extraction is untrusted: never promote invented citations or chat-only
	// details into topic knowledge. A source domain must exist in the exchange.
	allowed := map[string]bool{}
	for _, domain := range windSourceDomain.FindAllString(question+"\n"+answer, -1) {
		allowed[strings.TrimPrefix(strings.ToLower(domain), "www.")] = true
	}
	var facts []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		citation := windSourceCitation.FindStringSubmatch(l)
		if strings.HasPrefix(l, "-") && len(citation) > 1 && allowed[strings.TrimPrefix(strings.ToLower(citation[1]), "www.")] {
			facts = append(facts, l)
			if len(facts) == 4 {
				break
			}
		}
	}
	if len(facts) == 0 {
		return
	}
	_ = companion.AppendWindBrainIfRevision(s.windStateDir(), w.ID, revision, facts)
}

var windSourceDomain = regexp.MustCompile(`(?i)\b[a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,}\b`)
var windSourceCitation = regexp.MustCompile(`(?i)\[([a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,})\]`)

// windLearnLoop és la via d'extracció; els tests la substitueixen.
var windLearnLoop = func(s *Server, ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
	out, _, err := client.ChatFO(ctx, cfg.PrimTarget(p, role), cfg.FallbackTarget(role), msgs, role.Temperature, role.MaxTokens, nil)
	return out, err
}
