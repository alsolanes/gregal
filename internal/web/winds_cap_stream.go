package web

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// La resposta del Ventet a l'inici: el model escriu un JSON {action,…,reply}
// i, quan respon ell mateix («self»), el reply és la resposta sencera. Abans
// era una crida lateral de 2048 tokens sense flux: una pregunta llarga
// esperava 8-10 s davant d'una pantalla quieta i, si el JSON es tallava,
// sortia el JSON cru (amb \" i tot) retallat a 1200 caràcters.

// capMaxTokens és el pressupost de la resposta del Ventet: prou per a una
// explicació llarga sencera dins del JSON.
const capMaxTokens = 8192

type capTokenKey struct{}

// withCapToken fa que la crida del Ventet vagi en flux i passi cada fragment.
func withCapToken(ctx context.Context, fn func(string)) context.Context {
	return context.WithValue(ctx, capTokenKey{}, fn)
}

func capTokenOf(ctx context.Context) func(string) {
	fn, _ := ctx.Value(capTokenKey{}).(func(string))
	return fn
}

// capModelCall és la crida del Ventet: el rol petit si n'hi ha (amb el del
// vent de reserva), sense pensament, i en flux si el context ho demana.
func capModelCall(s *Server, ctx context.Context, msgs []llm.Message) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	if client == nil || cfg == nil {
		return "", errConst("no model configured")
	}
	prim, fb := cfg.PrimTarget(p, role), cfg.FallbackTarget(role)
	if _, sr, sp, ok := agent.SmallRole(cfg); ok {
		if small := cfg.PrimTarget(sp, sr); small != prim {
			orig := prim
			prim, fb, role = small, &orig, sr
		}
	}
	ctx, cancel := context.WithTimeout(llm.WithThink(ctx, "no"), 90*time.Second)
	defer cancel()
	var out string
	var err error
	if tok := capTokenOf(ctx); tok != nil {
		out, _, _, err = client.ChatStreamWithToolsFO(ctx, prim, fb, msgs, role.Temperature, capMaxTokens, nil, tok, nil, nil)
	} else {
		out, _, err = client.ChatFO(ctx, prim, fb, msgs, role.Temperature, capMaxTokens, nil)
	}
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", errConst("empty answer")
	}
	return out, nil
}

// capReplyStream treu el camp reply del JSON a mesura que el model l'escriu
// i en passa el text ja descodificat. Només surt si l'acció ja diu «self»
// (una entrega a un vent és una frase curta que arriba amb el done) o si el
// model ha contestat en prosa, sense JSON.
type capReplyStream struct {
	buf     strings.Builder
	emit    func(string)
	mode    int // 0 encara no se sap, 1 JSON, 2 prosa
	pos     int // dins buf: on continua el reply (mode 1) o la prosa
	inReply bool
	closed  bool
	sent    bool
}

func (c *capReplyStream) feed(tok string) {
	c.buf.WriteString(tok)
	raw := c.buf.String()
	if c.mode == 0 {
		t := strings.TrimLeft(raw, " \t\r\n")
		if t == "" {
			return
		}
		if t[0] == '{' || t[0] == '`' {
			c.mode = 1
		} else {
			c.mode = 2
			c.pos = len(raw) - len(t)
		}
	}
	if c.mode == 2 {
		if c.pos < len(raw) {
			c.out(raw[c.pos:])
			c.pos = len(raw)
		}
		return
	}
	if c.closed {
		return
	}
	if !c.inReply {
		start, ok := capReplyStart(raw)
		if !ok || !capSelfBefore(raw[:start]) {
			return
		}
		c.inReply, c.pos = true, start
	}
	text, next, closed := decodeJSONStringPrefix(raw, c.pos)
	c.pos, c.closed = next, closed
	if text != "" {
		c.out(text)
	}
}

func (c *capReplyStream) out(s string) {
	if !c.sent {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return
		}
	}
	c.sent = true
	c.emit(s)
}

// capReplyStart diu on comença el contingut de la cadena del camp "reply".
func capReplyStart(raw string) (int, bool) {
	for from := 0; ; {
		i := strings.Index(raw[from:], `"reply"`)
		if i < 0 {
			return 0, false
		}
		j := from + i + len(`"reply"`)
		k := j
		for k < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[k])) {
			k++
		}
		if k >= len(raw) {
			return 0, false
		}
		if raw[k] != ':' {
			from = j
			continue
		}
		k++
		for k < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[k])) {
			k++
		}
		if k >= len(raw) {
			return 0, false
		}
		if raw[k] != '"' {
			return 0, false
		}
		return k + 1, true
	}
}

func capSelfBefore(prefix string) bool {
	i := strings.Index(prefix, `"action"`)
	if i < 0 {
		return false
	}
	rest := strings.TrimLeft(prefix[i+len(`"action"`):], " \t\r\n:")
	return strings.HasPrefix(rest, `"self"`)
}

// decodeJSONStringPrefix descodifica una cadena JSON des de from fins on
// arriba el text: torna el text, on continuar i si la cadena ja s'ha tancat.
// Una seqüència d'escapament a mitges espera el fragment següent.
func decodeJSONStringPrefix(raw string, from int) (string, int, bool) {
	var b strings.Builder
	i := from
	for i < len(raw) {
		ch := raw[i]
		if ch == '"' {
			return b.String(), i + 1, true
		}
		if ch != '\\' {
			r, size := utf8.DecodeRuneInString(raw[i:])
			if r == utf8.RuneError && size <= 1 && !utf8.FullRuneInString(raw[i:]) {
				break // caràcter partit entre fragments
			}
			b.WriteString(raw[i : i+size])
			i += size
			continue
		}
		if i+1 >= len(raw) {
			break
		}
		switch raw[i+1] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
		case 'b', 'f':
		case '"', '\\', '/':
			b.WriteByte(raw[i+1])
		case 'u':
			if i+6 > len(raw) {
				return b.String(), i, false
			}
			v, err := strconv.ParseUint(raw[i+2:i+6], 16, 32)
			if err != nil {
				i += 6
				continue
			}
			r := rune(v)
			if r >= 0xD800 && r < 0xDC00 { // primera meitat d'un parell
				if i+12 > len(raw) {
					return b.String(), i, false
				}
				if raw[i+6] == '\\' && raw[i+7] == 'u' {
					if lo, err := strconv.ParseUint(raw[i+8:i+12], 16, 32); err == nil && lo >= 0xDC00 && lo < 0xE000 {
						b.WriteRune((r-0xD800)<<10 + (rune(lo) - 0xDC00) + 0x10000)
						i += 12
						continue
					}
				}
			}
			b.WriteRune(r)
			i += 6
			continue
		default:
			b.WriteByte(raw[i+1])
		}
		i += 2
	}
	return b.String(), i, false
}

// capReplyFromBroken salva el reply d'un JSON que no s'ha pogut llegir
// (tallat o mal tancat): el text descodificat, mai el JSON cru.
func capReplyFromBroken(raw string) (string, bool) {
	start, ok := capReplyStart(raw)
	if !ok {
		return "", false
	}
	text, _, _ := decodeJSONStringPrefix(raw, start)
	text = strings.TrimSpace(text)
	return text, text != ""
}
