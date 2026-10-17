package main

import (
	"io"
	"testing"
	"time"
)

// La línia de vida: quan el pare tanca l'stdin (o mor), el servidor fa els
// passos de tancament un sol cop i surt amb codi 0.
func TestServeShutdownLifelineRunsStepsOnce(t *testing.T) {
	r, w := io.Pipe()
	var order []string
	exited := make(chan int, 2)
	s := newServeShutdown(
		func() { order = append(order, "stop") },
		nil,
		func() { order = append(order, "release") },
	)
	s.exit = func(code int) { exited <- code }
	s.watchLifeline(r)
	select {
	case <-exited:
		t.Fatal("exited before stdin closed")
	case <-time.After(50 * time.Millisecond):
	}
	_ = w.Close()
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lifeline did not trigger shutdown")
	}
	s.run() // un segon senyal no repeteix res
	if len(order) != 2 || order[0] != "stop" || order[1] != "release" {
		t.Fatalf("steps: %v", order)
	}
	select {
	case <-exited:
		t.Fatal("exited twice")
	default:
	}
}
