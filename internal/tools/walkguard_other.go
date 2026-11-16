//go:build !windows

package tools

import "io/fs"

// Fora de Windows no hi ha un atribut comú per als fitxers només al núvol:
// la guarda és la de les carpetes de núvol i les extensions.
func onlineOnly(fs.FileInfo) bool { return false }
