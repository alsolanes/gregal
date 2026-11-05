package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gregal/internal/shell"
	"gregal/internal/tools"
)

// Sense bwrap (Windows, macOS, Linux sense bubblewrap) els membres d'una colla
// també han de poder provar el que fan: el shell corre a la seva còpia privada
// del projecte, sense aïllament del sistema. És el que has triat: més útil que
// una colla que no pot executar ni un test, amb aquests límits:
//   - el directori de treball és la còpia del seient, mai el projecte original
//     (el que s'hi escrigui fora dels fitxers assignats no s'integra);
//   - les mateixes ordres prohibides que el bash de l'agent;
//   - res d'instal·lar, baixar, esborrar fora o tocar el sistema (a una colla
//     no hi ha ningú per dir «Permet»): es denega i el model ho sap;
//   - sense claus a l'entorn (API keys, tokens, contrasenyes).
func (b crewToolBoundary) shellInCopy(ctx context.Context, command string) (string, []string, error) {
	if pat := tools.DeniedCommand(command); pat != "" {
		return "", nil, fmt.Errorf("ordre prohibida a la colla (%s)", pat)
	}
	home, _ := os.UserHomeDir()
	if r, risky := tools.Risky(command, b.workspace, home); risky {
		return "", nil, fmt.Errorf("NOT RUN: %s (%s) needs the user's approval and a crew cannot ask for it; do without it or say what is missing", r.Kind, r.What)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	prog, args := shell.Argv(shell.NormalitzaPathsWindows(command))
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Dir = b.workspace
	cmd.Env = crewShellEnv(os.Environ())
	var out limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return out.String(), nil, errors.New("shell timed out after 2 minutes")
		}
		return out.String(), nil, fmt.Errorf("shell failed: %w", err)
	}
	return out.String(), nil, nil
}

// crewShellEnv treu de l'entorn tot el que sembli una credencial.
func crewShellEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		up := strings.ToUpper(name)
		if strings.Contains(up, "KEY") || strings.Contains(up, "TOKEN") || strings.Contains(up, "SECRET") ||
			strings.Contains(up, "PASSWORD") || strings.Contains(up, "PASSWD") || strings.Contains(up, "CREDENTIAL") ||
			strings.HasPrefix(up, "GREGAL_") || up == "GH_ENTERPRISE_TOKEN" {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// crewShellConfined diu si aquesta màquina pot fer servir el shell aïllat.
func crewShellConfined() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("bwrap")
	return err == nil
}
