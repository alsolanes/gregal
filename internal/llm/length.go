package llm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// MaxTokensCap sostre del reintent automàtic per length: el pressupost no
// puja mai per sobre, encara que el rol en demani menys. Fins a tres
// reintents doblant: el raonament no és determinista i amb un de sol
// (l'antic comportament) un segon pensament llarg tornava a tallar.
// 65536 = HALOGEN_MAX_TOKENS_CAP del motor local: més enllà el mateix
// provider ho rebutja.
const MaxTokensCap = 65536

// maxLengthRetries és el sostre de reintents per length: 1 intent base +
// fins a 3 reintents (2x, 4x, 8x). Acotat perquè cada reintent amb un model
// de raonament crema un pensament sencer facturat.
const maxLengthRetries = 3

// LengthError és un torn buit per finish_reason=length: el model (sovint
// raonant: el pensament compta dins de max_tokens) no ha tingut espai per
// respondre. És recuperable: tornar-ho a provar amb més pressupost sol
// funcionar. MaxTokens és el pressupost DEL ROL (l'original), no el del
// reintent intern, perquè el missatge digui què cal canviar al config.
type LengthError struct {
	Reasoned  int // caràcters de pensament (>0 si és model de raonament)
	MaxTokens int
	Partial   bool // el model havia començat la resposta, però l'ha tallada
}

// StreamInterruptedError reports a stream closed before its final signal.
// Partial output may already be visible; only buffered calls can safely retry.
type StreamInterruptedError struct{}

type recoverTruncationKey struct{}

// WithRecoverTruncation opts a streamed agent turn into buffering its output
// until finish_reason is known. Truncated output can then be discarded and
// retried without duplicating text already shown to the user. Ordinary chat
// streaming remains live unless this option is set.
func WithRecoverTruncation(ctx context.Context) context.Context {
	return context.WithValue(ctx, recoverTruncationKey{}, true)
}

func recoverTruncation(ctx context.Context) bool {
	v, _ := ctx.Value(recoverTruncationKey{}).(bool)
	return v
}

func (*StreamInterruptedError) Error() string {
	return "el stream del model s'ha interromput abans del senyal final; la resposta pot estar incompleta"
}

// Retry only uncommitted model output; never replay already executed tools or
// live callbacks. Keep the existing prompt and token budget unchanged.
func retryInterruptedStream(ctx context.Context, buffered bool, do func() error) error {
	const attempts = 3
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := do()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var interrupted *StreamInterruptedError
		if !buffered || !errors.As(err, &interrupted) || attempt >= attempts {
			return err
		}
		wait := backoffFor(attempt, nil)
		if hook := retryHookFrom(ctx); hook != nil {
			hook(attempt, attempts, wait, err)
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (e *LengthError) Error() string {
	if e.Partial {
		return fmt.Sprintf("la resposta s'ha tallat perquè el model ha exhaurit max_tokens (%d); puja el límit del rol o tria un model amb més pressupost de sortida", e.MaxTokens)
	}
	if e.Reasoned > 0 {
		return fmt.Sprintf("s'ha quedat sense tokens raonant (%d caràcters de pensament) i no ha arribat a respondre: posa max_tokens del rol a %d (ara %d) o tria un model sense raonament", e.Reasoned, suggerit(e.Reasoned, e.MaxTokens), e.MaxTokens)
	}
	return fmt.Sprintf("s'ha quedat sense tokens abans d'escriure res: puja max_tokens del rol (ara %d)", e.MaxTokens)
}

// suggerit calcula un max_tokens que deixi respirar el raonament observat:
// la mateixa fórmula del salt (pensament observat més marge de resposta),
// amb un mínim del doble del pressupost actual perquè el consell no quedi
// mai per sota del que ja es té. És una indicació, no una mesura exacta:
// el pensament varia a cada intent.
func suggerit(reasonedChars, actual int) int {
	sug := saltRaonament(reasonedChars, 0)
	if doblat := dobla(actual); sug < doblat {
		sug = doblat
	}
	return sug
}

// lengthTallat construeix l'error d'un torn tallat per finish_reason=length
// decidint si és recuperable pel senyal que compta de veritat: el text de
// RESPOSTA que ha arribat al caller. Si n'hi ha (visible>0), ja s'ha emès i
// repetir-lo el duplicaria → queda marcat «parcial» i retryLength no el toca.
// Si només s'ha emès raonament (visible==0), a la pantalla no hi ha res de la
// resposta: el torn es pot tornar a provar amb el doble de pressupost, i el
// pensament —que sí que es repetiria— és admissible.
func lengthTallat(reasoned, maxTokens, visible int) *LengthError {
	return &LengthError{Reasoned: reasoned, MaxTokens: maxTokens, Partial: visible > 0}
}

// AsLengthError diu si l'error és per pressupost exhaurit (recuperable amb
// més max_tokens) o una altra cosa (configuració, xarxa, resposta absurda).
func AsLengthError(err error) (*LengthError, bool) {
	var le *LengthError
	if errors.As(err, &le) {
		return le, true
	}
	return nil, false
}

// retryLength torna a cridar doblant el pressupost (fins al sostre,
// maxLengthRetries cops) si l'error és LengthError. Cobreix el cas del
// transcript: model de raonament amb max_tokens curt que mata el torn
// sencer i, amb ell, tota la tasca. Només hi arriben els talls sense text
// de resposta visible (només raonament, o res): el reintent no duplica res
// que l'usuari hagi vist; el pensament sí que es pot repetir. Vegeu
// lengthTallat.
func retryLength(ctx context.Context, maxTokens int, do func(budget int) error) error {
	if err := do(maxTokens); err != nil {
		return escalaLength(ctx, err, maxTokens, do)
	}
	return nil
}

// dobla puja el pressupost al doble sense passar del sostre ni
// desbordar l'int.
func dobla(budget int) int {
	if budget <= 0 {
		return 0
	}
	if budget >= MaxTokensCap {
		return MaxTokensCap
	}
	if budget > MaxTokensCap/2 {
		return MaxTokensCap
	}
	return budget * 2
}

// escalaLength continua doblant el pressupost després d'un LengthError no
// parcial, fins a maxLengthRetries reintents o el sostre. Si cap intent
// hi cap, informa l'error ORIGINAL (pressupost del rol): és el que l'usuari
// pot canviar al config. Qualsevol altre error atura l'escalada i puja tal
// qual: només el length és recuperable amb més pressupost.
//
// Quan el tall ve de raonament i el pensament observat ja es menja mig
// pressupost (o més), el salt no és a cegues: segueix la lògica d'opencode
// (max_tokens = resposta + pressupost de pensament) i salta directament a
// cobrir el pensament OBSERVAT més marge de resposta. El doblatge simple
// falla quan el segon pensament surt més llarg que el primer: el raonament
// no és determinista.
func escalaLength(ctx context.Context, err error, provat int, do func(budget int) error) error {
	le, ok := AsLengthError(err)
	if !ok || le.Partial || provat <= 0 || provat >= MaxTokensCap || ctx.Err() != nil {
		return err
	}
	pressupost := provat
	for i := 0; i < maxLengthRetries; i++ {
		següent := dobla(pressupost)
		// El pensament observat ja es menja mig pressupost (o més): és
		// ell la causa del tall i doblar a cegues cremaria un intent.
		// En aquest cas se salta a cobrir-lo més marge de resposta.
		if obs := observat(err); obs/2 >= pressupost {
			if salt := saltRaonament(obs, següent); salt > següent {
				següent = salt
			}
		}
		if següent <= pressupost || següent > MaxTokensCap || ctx.Err() != nil {
			break
		}
		pressupost = següent
		if err2 := do(pressupost); err2 == nil {
			return nil
		} else {
			err = err2
		}
		if le2, ok := AsLengthError(err); !ok || le2.Partial {
			return err
		}
		if pressupost >= MaxTokensCap {
			break
		}
	}
	return firstLength(err, le)
}

// observat extreu els caràcters de pensament de l'últim LengthError, si
// n'hi ha. Zero = el tall no ve de raonament.
func observat(err error) int {
	if le, ok := AsLengthError(err); ok {
		return le.Reasoned
	}
	return 0
}

// saltRaonament calcula el pressupost que cobreix el pensament observat
// més marge per respondre (el doble del pensament estimat en tokens més
// 2048 de resposta), arrodonit a potència de dos i topat al sostre. Si el
// resultat no millora el doblatge, es queda el doblatge.
func saltRaonament(reasonedChars, doblat int) int {
	est := reasonedChars / 4
	if est < 0 {
		est = 0
	}
	want := est*2 + 2048
	salt := 4096
	for salt < want && salt < MaxTokensCap {
		salt *= 2
	}
	if salt < doblat {
		return doblat
	}
	return salt
}

// firstLength recupera l'error original quan l'escalada també acaba en
// length: el missatge ha de parlar del pressupost del rol, no del de
// l'últim reintent intern.
func firstLength(actual error, original *LengthError) error {
	if _, ok := AsLengthError(actual); ok {
		return original
	}
	return actual
}

// reDoesNotFit capta l'error dels motors que reserven prompt+max_tokens
// contra el context (Halogen: «max_tokens 65536 does not fit: prompt is
// 62192 tokens and the context is 122880, leaving room for 60688») i extreu
// la sala real que hi queda per a la resposta. Sense això, un max_tokens
// generós del rol mata tota petició amb prompt llarg en comptes d'adaptar-se.
var reDoesNotFit = regexp.MustCompile(`leaving room for (\d+)`)

// fitBudget torna el max_tokens més gran que cap al context segons l'error
// del provider (una mica per sota de la sala reportada, per al marge de
// tokens de control). Segon valor: si l'error no era un «does not fit».
func fitBudget(err error, maxTokens int) (int, bool) {
	if err == nil || maxTokens <= 0 {
		return 0, false
	}
	m := reDoesNotFit.FindStringSubmatch(err.Error())
	if len(m) < 2 {
		return 0, false
	}
	room, e := strconv.Atoi(m[1])
	if e != nil || room <= 0 {
		return 0, false
	}
	budget := room - 512
	if budget < 1024 {
		budget = 1024
	}
	if budget > maxTokens {
		budget = maxTokens
	}
	return budget, true
}

// retryFit reintenta una petició rebutjada per «max_tokens does not fit»
// amb el pressupost que el mateix provider diu que hi cap. Si no és un
// error de fit, delega en l'escalada per length (el camí de sempre), que
// parteix de l'últim pressupost provat per no repetir intents.
func retryFit(ctx context.Context, maxTokens int, do func(budget int) error) error {
	err := do(maxTokens)
	if err == nil || ctx.Err() != nil {
		return err
	}
	provat := maxTokens
	if budget, ok := fitBudget(err, maxTokens); ok && budget < maxTokens {
		if err2 := do(budget); err2 == nil {
			return nil
		} else {
			err = err2
			provat = budget
		}
	}
	return escalaLength(ctx, err, provat, do)
}
