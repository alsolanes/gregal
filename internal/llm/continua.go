package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MaxContinuacions és el sostre de torns «continua» automàtics quan una
// resposta es talla per max_tokens amb feina feta (text o crides). Cada
// continuació torna a enviar l'historial sencer: més enllà d'aquest nombre
// el model segurament està en bucle i és millor lliurar el que hi ha.
const MaxContinuacions = 3

// TruncationNotice és l'avís que es passa pel RetryHook quan una resposta
// tallada per max_tokens es continua o es reintenta. No és un error de
// debò: és el canal que totes les superfícies (TUI, -p, web, Telegram) ja
// pinten, i RetryNote el reconeix per mostrar-lo tal qual.
type TruncationNotice struct {
	N, Max    int    // continuació en curs i sostre
	Tool      string // eina amb arguments incomplets (buit = text tallat)
	Budget    int    // max_tokens de la continuació
	Exhausted bool   // s'ha arribat al sostre: es lliura el que hi ha
}

func (n *TruncationNotice) Error() string {
	switch {
	case n.Exhausted:
		return fmt.Sprintf("resposta tallada per max_tokens %d cops: es conserva el text parcial", n.Max+1)
	case n.Tool != "":
		return fmt.Sprintf("crida a %s tallada per max_tokens (no s'executa) · reintent %d/%d amb max_tokens %d", n.Tool, n.N, n.Max, n.Budget)
	default:
		return fmt.Sprintf("resposta tallada per max_tokens · continuant %d/%d", n.N, n.Max)
	}
}

// teSortida diu si el tall porta feina que no es pot llençar: text de
// resposta o crides d'eina (encara que siguin a mitges).
func (e *LengthError) teSortida() bool {
	return e != nil && (strings.TrimSpace(e.Text) != "" || len(e.Calls) > 0)
}

// amb adjunta al tall el que el model havia arribat a escriure.
func (e *LengthError) amb(text string, calls []ToolCall) *LengthError {
	e.Text, e.Calls = text, calls
	return e
}

// argsComplets diu si els arguments d'una crida són un objecte JSON sencer.
// Un tall per max_tokens a mig argument deixa JSON invàlid; buit també
// compta com a incomplet, perquè amb length vol dir que el tall ha arribat
// just després del nom.
func argsComplets(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(raw, "{") && json.Valid([]byte(raw))
}

const promptContinua = "Your previous reply was cut off by the output token limit. Continue exactly where you stopped: do not repeat anything you already wrote and do not add any preamble."

func promptCridaTallada(tool string) string {
	return fmt.Sprintf("Your previous reply hit the output token limit while you were writing a call to the tool `%s`. The call was cut off and was NOT executed; no tool call from that reply was executed. Do not repeat the text you already wrote. Issue the tool call again, and if its arguments are large (for example a long file), split the work into several smaller calls (e.g. write a first part, then add the rest with further edits or appends) so each call fits in the output limit.", tool)
}

// continuaTallat embolcalla una crida al model perquè una resposta tallada
// per max_tokens no es perdi mai:
//
//   - el text parcial es conserva i es demana un torn «continua» que
//     l'allarga, fins a MaxContinuacions cops;
//   - una crida d'eina amb arguments incomplets no s'executa mai: es
//     descarta i es torna a demanar amb el doble de pressupost (fins al
//     sostre) i la indicació de partir la feina en trossos;
//   - si el tall arriba amb totes les crides senceres, es donen per bones;
//   - si s'esgoten les continuacions, o una continuació falla, es lliura
//     el text acumulat amb una nota, en comptes d'un error que el llençaria.
//
// onToken rep el text dels trossos que la crida no havia ensenyat (mode
// recuperació o resposta no-streaming) perquè la pantalla i el retorn
// coincideixin. Cada pas s'avisa pel RetryHook amb un *TruncationNotice.
func continuaTallat(ctx context.Context, msgs []Message, maxTokens int, onToken func(string), do func(msgs []Message, budget int) (string, []ToolCall, error)) (string, []ToolCall, error) {
	var acc strings.Builder
	cur, budget := msgs, maxTokens
	emet := func(s string) {
		if s != "" && onToken != nil {
			onToken(s)
		}
	}
	for n := 0; ; n++ {
		text, calls, err := do(cur, budget)
		if err == nil {
			acc.WriteString(text)
			return acc.String(), calls, nil
		}
		le, ok := AsLengthError(err)
		if !ok || !le.teSortida() {
			if acc.Len() == 0 {
				// Primer intent: el que digui la crida, tal qual (algunes
				// vies tornen text útil al costat de l'error).
				return text, calls, err
			}
			if ctx.Err() != nil {
				return acc.String(), nil, err
			}
			// Una continuació ha fallat però ja hi ha feina: es lliura
			// amb el motiu, no es llença.
			nota := fmt.Sprintf("\n\n[resposta incompleta: la continuació ha fallat (%v)]", err)
			emet(nota)
			acc.WriteString(nota)
			return acc.String(), nil, nil
		}
		if !le.Partial {
			emet(le.Text)
		}
		acc.WriteString(le.Text)
		trencada := ""
		for _, c := range le.Calls {
			if !argsComplets(c.Function.Arguments) {
				trencada = c.Function.Name
				if trencada == "" {
					trencada = "(sense nom)"
				}
				break
			}
		}
		if len(le.Calls) > 0 && trencada == "" {
			// Tallat just després de crides senceres: són executables.
			return acc.String(), le.Calls, nil
		}
		hook := retryHookFrom(ctx)
		if n >= MaxContinuacions || ctx.Err() != nil {
			if ctx.Err() != nil {
				return acc.String(), nil, ctx.Err()
			}
			if hook != nil {
				hook(n, MaxContinuacions, 0, &TruncationNotice{N: n, Max: MaxContinuacions, Exhausted: true})
			}
			nota := fmt.Sprintf("\n\n[resposta tallada: s'ha arribat al límit de sortida (max_tokens %d) després de %d continuacions]", budget, n)
			if trencada != "" {
				nota = fmt.Sprintf("\n\n[resposta tallada: la crida a %s ha quedat incompleta per max_tokens i no s'ha executat]", trencada)
			}
			emet(nota)
			acc.WriteString(nota)
			return acc.String(), nil, nil
		}
		prompt := promptContinua
		if trencada != "" {
			prompt = promptCridaTallada(trencada)
			if d := dobla(budget); d > budget {
				budget = d
			}
		}
		if hook != nil {
			hook(n+1, MaxContinuacions, 0, &TruncationNotice{N: n + 1, Max: MaxContinuacions, Tool: trencada, Budget: budget})
		}
		cur = append([]Message(nil), msgs...)
		if s := acc.String(); strings.TrimSpace(s) != "" {
			cur = append(cur, Message{Role: "assistant", Content: s})
		}
		cur = append(cur, Message{Role: "user", Content: prompt})
	}
}
