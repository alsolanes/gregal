package web

import (
	"embed"
	_ "embed"
)

//go:embed index.html
var indexHTML []byte

// assets són els mòduls de la UI que ja no caben a index.html. La pàgina
// és prou gran: tot el que s'hi afegeix de nou (pestanyes de sessió,
// canvis, fitxers, terminal, composer) viu a app/ com a mòdul ES i
// s'hi carrega amb <script type="module">.
//
//go:embed app
var assets embed.FS
