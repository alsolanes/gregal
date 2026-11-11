package runs

import (
	"context"
	"errors"
	"testing"
)

// TestWaitingComputerParksResumesAndFails (B6): un torn que necessita
// l'ordinador s'aparca a waiting_computer sense ocupar el worker (un altre
// torn de la mateixa sessió corre mentrestant), es reprèn amb el mateix ID
// quan l'ordinador respon i, si no arrenca, Fail el tanca amb el motiu.
func TestWaitingComputerParksResumesAndFails(t *testing.T) {
	q := New()
	run, _, err := q.Submit(func(ctx context.Context, r Run) error {
		return ErrWaitingComputer
	}, Run{Session: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if got := waitDone(t, q, run.ID); got.State != WaitingComputer {
		t.Fatalf("estat %s, volia waiting_computer", got.State)
	}
	// El worker és lliure: un altre torn de la sessió acaba.
	other, _, _ := q.Submit(func(ctx context.Context, r Run) error { return nil }, Run{Session: "s"})
	if got := waitDone(t, q, other.ID); got.State != Completed {
		t.Fatalf("el worker ha quedat ocupat: %s", got.State)
	}
	resumed := make(chan int64, 1)
	if _, ok := q.Resume(run.ID, func(ctx context.Context, r Run) error { resumed <- r.ID; return nil }); !ok {
		t.Fatal("Resume ha refusat un torn a waiting_computer")
	}
	if got := waitDone(t, q, run.ID); got.State != Completed || <-resumed != run.ID {
		t.Fatalf("represa: %+v", got)
	}
	if _, ok := q.Resume(run.ID, func(ctx context.Context, r Run) error { return nil }); ok {
		t.Fatal("dues represes del mateix torn")
	}

	// Ordinador que no arrenca: estat honest.
	second, _, _ := q.Submit(func(ctx context.Context, r Run) error { return ErrWaitingComputer }, Run{Session: "s"})
	waitDone(t, q, second.ID)
	if !q.Fail(second.ID, "l'ordinador no ha arrencat") {
		t.Fatal("Fail ha refusat un torn aparcat")
	}
	got, _ := q.Get(second.ID)
	if got.State != Failed || got.Err != "l'ordinador no ha arrencat" {
		t.Fatalf("Fail: %+v", got)
	}
	if q.Fail(second.ID, "x") {
		t.Fatal("Fail d'un torn ja tancat")
	}
	// Adopció després d'un reinici, i cancel·lació d'un aparcat.
	adopted := q.Park(Run{ID: 900, Session: "s", State: WaitingComputer})
	if adopted.State != WaitingComputer {
		t.Fatalf("Park ha perdut l'estat: %s", adopted.State)
	}
	if !q.Cancel(900) {
		t.Fatal("no es pot cancel·lar un torn esperant l'ordinador")
	}
	if got, _ := q.Get(900); got.State != Cancelled {
		t.Fatalf("cancel·lat: %s", got.State)
	}
	if !errors.Is(ErrWaitingComputer, ErrWaitingComputer) || ErrWaitingComputer == ErrWaitingUser {
		t.Fatal("errors d'espera confosos")
	}
}
