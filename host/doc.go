// Package host és l'única porta pública de Gregal per a productes que viuen
// en un altre repositori (Gregal Jocs, a alsolanes/gregal-jocs). Cada
// subpaquet reexporta, amb àlies, només el que aquests productes fan servir
// d'internal/; la resta de Gregal es queda privat.
//
// Si un producte necessita alguna cosa més, s'afegeix aquí (un àlies, no
// una còpia) i es comprova amb `go build ./...` a tots dos repositoris.
package host
