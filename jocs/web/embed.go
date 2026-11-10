// Package web són els fitxers web de Gregal Jocs (la pàgina /jocs/, l'Arena,
// la lliga dels vents i les seves llibreries). jocs/server els registra com
// a fitxers d'un producte: el servidor de Gregal els serveix a /app/<nom>,
// a les mateixes URL de sempre.
package web

import "embed"

// Assets té app/<nom>: arena*.js, football*.js, jocs-site.*, els visors i
// vendor/ (three.js i mp4-muxer, que només fan servir els jocs).
//
//go:embed app
var Assets embed.FS
