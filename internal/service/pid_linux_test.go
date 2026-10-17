package service

import (
	"os"
	"os/exec"
	"testing"
)

func TestSameProgramRecognisesItselfButNotAnotherBinary(t *testing.T) {
	if !sameProgram(os.Getpid()) {
		t.Fatal("own PID not recognised")
	}
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skip("no sleep binary:", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if sameProgram(cmd.Process.Pid) {
		t.Fatal("a recycled PID running another program counts as the server")
	}
}
