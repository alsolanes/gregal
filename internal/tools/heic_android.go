//go:build android

package tools

import (
	"errors"
)

// A Android el servidor local no porta el descodificador HEIC (depèn de
// càrrega dinàmica de llibreries que no hi ha sense cgo). L'app ja converteix
// les fotos a JPEG abans de pujar-les; aquí només es detecten per donar un
// error clar.

func IsHEIF(raw []byte) bool {
	if len(raw) < 12 || string(raw[4:8]) != "ftyp" {
		return false
	}
	switch string(raw[8:12]) {
	case "heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs", "mif1", "msf1":
		return true
	}
	return false
}

func HEIFToJPEG([]byte) ([]byte, error) {
	return nil, errors.New("no puc llegir la foto HEIC en aquest dispositiu: converteix-la a JPEG")
}
