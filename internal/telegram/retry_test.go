package telegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// clientAmb reintents: el transport que fa servir el bot de debò.
func clientAmb(handler http.HandlerFunc) (*http.Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return &http.Client{Timeout: 10 * time.Second, Transport: retryTransport{base: srv.Client().Transport}}, srv
}

// TestRetryAguantaUn429: Telegram respon 429 quan fem massa edicions seguides
// (el streaming n'és el cas típic) i diu quants segons esperar. El bot no ha
// de veure cap error: ha de tornar a provar i acabar bé.
func TestRetryAguantaUn429(t *testing.T) {
	var intents int32
	c, srv := clientAmb(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&intents, 1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()
	resp, err := c.Post(srv.URL, "application/json", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatalf("no hauria de fallar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("codi = %d, esperava 200 després del reintent", resp.StatusCode)
	}
	if n := atomic.LoadInt32(&intents); n != 2 {
		t.Fatalf("intents = %d, esperava 2", n)
	}
}

// TestRetryAguantaUn500 i torna a enviar el cos sencer (si el cos es perdés,
// Telegram respondria «Bad Request» i l'usuari no rebria res).
func TestRetryAguantaUn500(t *testing.T) {
	var intents int32
	var cossos []string
	c, srv := clientAmb(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		cossos = append(cossos, string(b))
		if atomic.AddInt32(&intents, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()
	resp, err := c.Post(srv.URL, "application/json", strings.NewReader(`{"text":"hola"}`))
	if err != nil {
		t.Fatalf("no hauria de fallar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("codi = %d", resp.StatusCode)
	}
	// El cos ha d'arribar sencer a TOTS els intents, no nomes al reintent:
	// si el primer surt buit, Telegram respon 400 i l'usuari no rep res.
	if len(cossos) != 2 {
		t.Fatalf("intents = %d, esperava 2", len(cossos))
	}
	for i, cos := range cossos {
		if cos != `{"text":"hola"}` {
			t.Fatalf("cos de l'intent %d = %q", i+1, cos)
		}
	}
}

// TestRetryNoInsisteixAmbUn400: un error de debò (petició mal feta) no s'ha
// de reintentar: reintentar-lo només fa esperar l'usuari per no res.
func TestRetryNoInsisteixAmbUn400(t *testing.T) {
	var intents int32
	c, srv := clientAmb(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&intents, 1)
		w.WriteHeader(http.StatusBadRequest)
	})
	defer srv.Close()
	resp, _ := c.Post(srv.URL, "application/json", strings.NewReader(`{}`))
	resp.Body.Close()
	if n := atomic.LoadInt32(&intents); n != 1 {
		t.Fatalf("intents = %d, esperava 1 (sense reintents)", n)
	}
}

// TestRetryLimitaLEspera: si Telegram demana una espera enorme, no ens hi
// quedem: val més fallar de pressa i poder dir-ho.
func TestRetryLimitaLEspera(t *testing.T) {
	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"600"}},
		Body:       http.NoBody,
	}
	if w := retryAfter(resp); w != maxRetryWait {
		t.Fatalf("espera = %v, esperava el màxim %v", w, maxRetryWait)
	}
}

// TestRetrySenseCosReplicable: si el cos no es pot repetir, no s'ha de
// reintentar (enviar-lo a mitges seria pitjor que fallar).
func TestRetrySenseCosReplicable(t *testing.T) {
	var intents int32
	c, srv := clientAmb(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&intents, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer srv.Close()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", srv.URL, io_NoGetBody{})
	req.GetBody = nil
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if n := atomic.LoadInt32(&intents); n != 1 {
		t.Fatalf("intents = %d, esperava 1", n)
	}
}

// io_NoGetBody simula un cos que no es pot replicar.
type io_NoGetBody struct{}

func (io_NoGetBody) Read(p []byte) (int, error) { return 0, io.EOF }
func (io_NoGetBody) Close() error               { return nil }
