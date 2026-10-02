package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestEdicionsLimitades: el streaming edita el missatge a cada tros de
// resposta i Telegram respon 429 a les edicions seguides (el limitador
// existeix per no arribar-hi mai). Pero el text NO es pot perdre: si es
// limités el text i no el ritme, l'usuari veuria una resposta escapçada.
func TestEdicionsLimitades(t *testing.T) {
	var edits, enviaments int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "sendMessage"):
			atomic.AddInt32(&enviaments, 1)
			w.Write([]byte(`{"ok":true,"result":{"message_id":7,"chat":{"id":1},"text":"x"}}`))
		case strings.Contains(r.URL.Path, "editMessageText"):
			atomic.AddInt32(&edits, 1)
			w.Write([]byte(`{"ok":true,"result":{"message_id":7,"chat":{"id":1},"text":"x"}}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()
	b := &Bot{api: &API{Token: "1:aaa", BaseURL: srv.URL, HTTP: srv.Client()}, logf: func(string, ...any) {}}
	ctx := context.Background()
	var id int
	var buf strings.Builder

	// El primer tros envia el missatge: es la resposta immediata que veu
	// l'usuari, i aixo no es limita mai.
	b.streamAnswer(ctx, 1, &id, &buf, "primer")
	if n := atomic.LoadInt32(&enviaments); n != 1 {
		t.Fatalf("enviaments = %d, esperava 1", n)
	}
	if n := atomic.LoadInt32(&edits); n != 0 {
		t.Fatalf("edits = %d, encara no toca", n)
	}

	// Cinc trossos seguits: cap edicio (tots dins la finestra).
	for i := 0; i < 5; i++ {
		b.streamAnswer(ctx, 1, &id, &buf, "tros")
	}
	if n := atomic.LoadInt32(&edits); n != 0 {
		t.Fatalf("ha editat %d cops dins la finestra", n)
	}
	// Pero el text hi es tot: res no s'ha perdut.
	if got := strings.Count(buf.String(), "tros"); got != 5 {
		t.Fatalf("el buffer te %d trossos, esperava 5", got)
	}
	if !strings.Contains(buf.String(), "primer") {
		t.Fatal("el buffer ha perdut el primer tros")
	}

	// Passada la finestra, torna a editar.
	time.Sleep(editInterval + 100*time.Millisecond)
	b.streamAnswer(ctx, 1, &id, &buf, "final")
	if n := atomic.LoadInt32(&edits); n != 1 {
		t.Fatalf("edits = %d, esperava 1", n)
	}
	// I el missatge editat porta tot el text acumulat.
	text := renderAnswer(buf.String())
	for _, tros := range []string{"primer", "final"} {
		if !strings.Contains(text, tros) {
			t.Fatalf("el text editat no conte %q", tros)
		}
	}
}

// TestLimitadorEsPerXat: dos xats diferents no es fan esperar l'un a l'altre.
func TestLimitadorEsPerXat(t *testing.T) {
	b := &Bot{}
	if !b.potEditar(1) {
		t.Fatal("la primera edicio s'ha de permetre")
	}
	if b.potEditar(1) {
		t.Fatal("la segona seguida no s'ha de permetre")
	}
	if !b.potEditar(2) {
		t.Fatal("un altre xat no hi te res a veure")
	}
}
