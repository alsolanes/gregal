package web

import (
	"context"
	"testing"
	"time"
)

// El límit d'espera d'una aprovació surt del config, arriba a la UI com a
// timeout_s i la cancel·lació del torn el talla. Abans eren 120 s fixos,
// escrits a mà al servidor i a la UI, i un torn aturat amb una aprovació
// pendent quedava penjat fins al final del compte.
func TestAprovacioLimitDelConfig(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Agent.ApprovalTimeoutS = 1
	var enviat map[string]any
	emit := func(ev string, v any) {
		if ev == "approve_request" {
			enviat, _ = v.(map[string]any)
		}
	}
	t0 := time.Now()
	ok, caducada := s.waitApproval(context.Background(), emit, "c1", "edit", "{}", "sig", false)
	if ok || !caducada {
		t.Fatalf("sense resposta en 1 s ha de caducar: ok=%v caducada=%v", ok, caducada)
	}
	if d := time.Since(t0); d > 3*time.Second {
		t.Fatalf("ha esperat %s amb un límit d'1 s", d)
	}
	if enviat["timeout_s"] != "1" || enviat["auto_approve_allowed"] != false {
		t.Fatalf("la UI ha de rebre el límit real: %v", enviat)
	}
}

func TestAprovacioSenseLimitIAturada(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Agent.ApprovalTimeoutS = -1
	var enviat map[string]any
	emit := func(ev string, v any) {
		if ev == "approve_request" {
			enviat, _ = v.(map[string]any)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	t0 := time.Now()
	ok, caducada := s.waitApproval(ctx, emit, "c1", "edit", "{}", "sig", true)
	if ok || caducada {
		t.Fatalf("aturar el torn no és ni permís ni caducitat: ok=%v caducada=%v", ok, caducada)
	}
	if d := time.Since(t0); d > 2*time.Second {
		t.Fatalf("la cancel·lació havia de tallar l'espera, ha trigat %s", d)
	}
	if enviat["timeout_s"] != "0" || enviat["auto_approve_allowed"] != true {
		t.Fatalf("sense límit, la UI rep 0: %v", enviat)
	}
	s.mu.Lock()
	n := len(s.approvals)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("l'aprovació aturada no pot quedar pendent (%d)", n)
	}
}

func TestAprovacioLimitPerDefecte(t *testing.T) {
	s := goalTestServer(t)
	if d := s.cfg.ApprovalTimeout(); d != 30*time.Minute {
		t.Fatalf("sense config, 30 minuts: %s", d)
	}
}
