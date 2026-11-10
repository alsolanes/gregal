package arena

// Playbook és la guia «què guanya a què» per a qui escriu el .md: les
// opcions de cada decisió explicades en llenguatge planer i la taula de
// contrajoc del motor. Es genera de les mateixes taules que fa servir el
// motor, de manera que la UI no pot dir una cosa i el partit fer-ne una altra.
type Playbook struct {
	Sections []PlaySection `json:"sections"`
}

// PlaySection és una decisió amb contrajoc: les meves opcions (files) contra
// les del rival (columnes).
type PlaySection struct {
	ID     string       `json:"id"`
	Title  string       `json:"title"`
	Intro  string       `json:"intro"`
	Mine   []PlayOption `json:"mine"`
	Theirs []PlayOption `json:"theirs"`
	// Grid[i][j]: com li va la meva opció i contra la seva j, de -2 (perds
	// clarament) a +2 (guanyes clarament).
	Grid [][]int `json:"grid"`
	// Values, si n'hi ha: el número del motor que hi ha darrere de cada
	// casella (p. ex. el % de rondes guanyades al CS2), i Unit diu què és.
	Values [][]int `json:"values,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	// Rule és la plantilla de la regla que la UI insereix al .md: {theirs}
	// i {mine} es canvien pels ids de la columna i la fila.
	Rule string `json:"rule"`
}

// PlayOption és una opció del motor: l'id exacte que va al JSON, un nom
// curt i què fa (per a qui no coneix el joc).
type PlayOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Plain string `json:"plain"`
}

// Playbooker és el joc que en té (tots tres en tenen; és opcional perquè
// un joc nou pugui arribar sense).
type Playbooker interface {
	Playbook() Playbook
}

// Grade passa un valor a l'escala -2..+2 amb quatre llindars creixents.
func Grade(v int, t [4]int) int {
	switch {
	case v < t[0]:
		return -2
	case v < t[1]:
		return -1
	case v <= t[2]:
		return 0
	case v <= t[3]:
		return 1
	}
	return 2
}
