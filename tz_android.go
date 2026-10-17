//go:build android

package main

import (
	"os"
	"time"
	_ "time/tzdata"
)

// A Android, Go deixa time.Local sempre en UTC (initLocal és un TODO de la
// biblioteca estàndard) i no troba el tzdata, que en les versions noves viu
// a l'APEX. Les hores de les converses sortien en UTC i l'app, que les llegeix
// com a hora local, deia «fa 2 h» d'una conversa de fa un minut. L'app passa
// la zona del telèfon a TZ; el tzdata incrustat la resol.
func init() {
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			time.Local = loc
		}
	}
}
