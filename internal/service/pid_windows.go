//go:build windows

package service

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func processAlive(pid int) bool {
	// tasklist és present a Windows suportat i no requereix dependències CGO.
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	if err != nil {
		// No podem distingir procés mort d'una consulta fallida; conservar el
		// lock és segur i evita dos escriptors.
		return true
	}
	needle := strconv.Itoa(pid)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == needle {
			return true
		}
	}
	return false
}
