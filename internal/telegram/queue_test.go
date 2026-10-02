package telegram

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/runs"
)

func TestTelegramQueueSerialitzaElXat(t *testing.T) {
	b := &Bot{queue: runs.New()}
	started := make(chan struct{})
	release := make(chan struct{})
	var active atomic.Int32
	var max atomic.Int32
	exec := func(ctx context.Context, _ runs.Run) error {
		n := active.Add(1)
		for {
			old := max.Load()
			if n <= old || max.CompareAndSwap(old, n) {
				break
			}
		}
		closeOnce(started)
		select {
		case <-release:
		case <-ctx.Done():
			return runs.ErrCancelled
		}
		active.Add(-1)
		return nil
	}
	first, _, err := b.queue.Submit(exec, runs.Run{Session: tgScope(42), Workspace: runs.WorkspaceKey("project")})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := b.queue.Submit(exec, runs.Run{Session: tgScope(42), Workspace: runs.WorkspaceKey("project")})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("el primer torn no ha començat")
	}
	if max.Load() != 1 {
		t.Fatalf("hi ha hagut execucions simultànies: màxim %d", max.Load())
	}
	close(release)
	waitTelegramRun(t, b.queue, first.ID)
	waitTelegramRun(t, b.queue, second.ID)
	if max.Load() != 1 {
		t.Fatalf("la sessió ha executat torns en paral·lel: màxim %d", max.Load())
	}
}

func TestTelegramQueueCancelTornEnCua(t *testing.T) {
	b := &Bot{queue: runs.New()}
	started := make(chan struct{})
	release := make(chan struct{})
	block := func(ctx context.Context, _ runs.Run) error {
		closeOnce(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return runs.ErrCancelled
		}
	}
	first, _, _ := b.queue.Submit(block, runs.Run{Session: tgScope(99)})
	second, _, _ := b.queue.Submit(func(context.Context, runs.Run) error {
		t.Fatal("un torn cancel·lat no s'ha d'executar")
		return nil
	}, runs.Run{Session: tgScope(99)})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("el primer torn no ha començat")
	}
	if !b.queue.Cancel(second.ID) {
		t.Fatal("la cancel·lació del torn en cua hauria de donar cert")
	}
	close(release)
	if got := waitTelegramRun(t, b.queue, first.ID); got.State != runs.Completed {
		t.Fatalf("primer estat: %s", got.State)
	}
	if got := waitTelegramRun(t, b.queue, second.ID); got.State != runs.Cancelled {
		t.Fatalf("segon estat: %s", got.State)
	}
}

func waitTelegramRun(t *testing.T, q *runs.Queue, id int64) runs.Run {
	t.Helper()
	select {
	case <-q.Done(id):
		r, ok := q.Get(id)
		if !ok {
			t.Fatalf("execució %d desapareguda", id)
		}
		return r
	case <-time.After(2 * time.Second):
		t.Fatalf("execució %d no ha acabat", id)
		return runs.Run{}
	}
}

// closeOnce evita que un executor de prova tanqui dues vegades el canal si
// la cua canvia de torn abans que el test hagi llegit l'event.
func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
