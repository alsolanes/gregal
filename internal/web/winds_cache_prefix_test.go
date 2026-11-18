package web

import (
	"strings"
	"testing"
	"time"

	"gregal/internal/llm"
)

// Les notes apreses en segon pla no canvien el system a mitja conversa: es
// mantenen mentre només creixen i es refan si la persona n'esborra una o si
// canvia la conversa.
func TestFrozenWindNotesStayWhileTheyOnlyGrow(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	conv := windConversationKey(nil, "Vull comprar un portàtil")
	v1 := map[string]string{"memory": "\n- Programa en Go"}
	if got := s.frozenWindNotes("ajudant", conv, v1); got["memory"] != v1["memory"] {
		t.Fatalf("primer torn: %q", got["memory"])
	}
	// El vent n'aprèn una de nova: el system no es mou.
	v2 := map[string]string{"memory": "\n- Programa en Go\n- Pressupost 1.500 €"}
	if got := s.frozenWindNotes("ajudant", conv, v2); got["memory"] != v1["memory"] {
		t.Fatalf("una nota apresa ha canviat el system a mitja conversa: %q", got["memory"])
	}
	// La persona n'esborra una: es nota de seguida.
	v3 := map[string]string{"memory": "\n- Pressupost 1.500 €"}
	if got := s.frozenWindNotes("ajudant", conv, v3); got["memory"] != v3["memory"] {
		t.Fatalf("una nota esborrada encara surt: %q", got["memory"])
	}
	// Conversa nova: es tornen a llegir.
	v4 := map[string]string{"memory": "\n- Pressupost 1.500 €\n- Viu a Girona"}
	if got := s.frozenWindNotes("ajudant", windConversationKey(nil, "Una altra cosa"), v4); got["memory"] != v4["memory"] {
		t.Fatalf("conversa nova amb notes velles: %q", got["memory"])
	}
}

// La clau de la conversa és la mateixa al primer torn (el missatge és el nou)
// i als següents (ja és el primer de l'historial).
func TestWindConversationKeyIsStableFromTheFirstTurn(t *testing.T) {
	first := windConversationKey(nil, "Hola, vull un portàtil")
	later := windConversationKey([]llm.Message{
		{Role: "user", Content: "Hola, vull un portàtil", Ts: 1},
		{Role: "assistant", Content: "D'acord"},
	}, "I amb 32 GB?")
	if first != later {
		t.Fatalf("la conversa canvia de clau al segon torn")
	}
}

// El que s'envia en un torn és, sencer, l'inici del que s'envia al següent:
// el missatge de l'usuari torna amb la mateixa hora i l'historial no llisca
// un torn cada vegada.
func TestWindTurnPrefixIsStableAcrossTurns(t *testing.T) {
	windTurnsAnchorMu.Lock()
	windTurnsAnchors = map[string]time.Time{}
	windTurnsAnchorMu.Unlock()
	base := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	var history []llm.Message
	var prev []llm.Message
	breaks := 0
	for turn := 0; turn < 30; turn++ {
		start := base.Add(time.Duration(turn) * 3 * time.Minute)
		msg := "pregunta " + strings.Repeat("p", 300) + " " + string(rune('a'+turn%26))
		sent := windTurnMessagesAt("SYS", history, msg, start)
		if prev != nil {
			// Tot el que es va enviar al torn anterior, excepte la resposta que
			// encara no hi era, ha de ser el principi d'aquest.
			if !messagesPrefix(prev, sent) {
				breaks++
			}
		}
		history = append(history,
			llm.Message{Role: "user", Content: msg, Ts: start.Unix()},
			llm.Message{Role: "assistant", Content: "resposta " + strings.Repeat("r", 1500)})
		prev = sent
	}
	t.Logf("trencaments del prefix en 30 torns: %d", breaks)
	// Sense el retall per blocs, a partir del torn ~8 es trencava a cada torn.
	if breaks > 6 {
		t.Fatalf("el prefix s'ha trencat %d cops en 30 torns", breaks)
	}
}

func messagesPrefix(a, b []llm.Message) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i].Role != b[i].Role || a[i].Content != b[i].Content {
			return false
		}
	}
	return true
}

// Cada missatge de l'usuari de l'historial porta l'hora amb què es va enviar.
func TestWindChatTurnsKeepTheSentClockTag(t *testing.T) {
	at := time.Date(2026, 11, 18, 9, 21, 0, 0, time.UTC)
	sent := windTurnMessagesAt("SYS", nil, "Quina hora és?", at)
	turns := windChatTurns([]llm.Message{{Role: "user", Content: "Quina hora és?", Ts: at.Unix()}, {Role: "assistant", Content: "Les 10:21"}})
	if turns[0].Content != sent[1].Content {
		t.Fatalf("el missatge torna diferent de com es va enviar:\n%q\n%q", turns[0].Content, sent[1].Content)
	}
}
