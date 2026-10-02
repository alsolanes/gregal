// Package tema és la paleta de Gregal com a dades, per als tres fronts.
//
// Fins ara els colors vivien copiats a `internal/tui/theme.go`, al `:root`
// d'`internal/web/index.html` i a `desktop/main.js`, amb dos tests que
// comparaven les còpies amb `docs/identitat.md` perquè no divergissin. Els
// tests feien la seva feina, però tres còpies segueixen sent tres llocs on
// canviar un color, i cap dels tres sabia res dels colors de sintaxi del
// codi: el TUI pintava amb l'estil per defecte de glamour (d'una altra
// marca) i el web no pintava el codi de cap manera.
//
// Aquí hi ha un sol joc de tokens per tema, i els derivats surten d'ell:
// els estils de lipgloss (TUI), el StyleConfig de glamour (markdown al
// terminal), l'estil de chroma (codi als dos fronts) i el bloc :root del
// CSS (web i escriptori).
package tema

// Sintaxi són els colors del codi. Vuit categories i prou: chroma en
// distingeix desenes, però amb més de vuit colors un bloc de codi deixa
// de llegir-se i sembla un arbre de Nadal.
type Sintaxi struct {
	Paraula   string // if, func, return
	Cadena    string // "text"
	Numero    string // 42, 0x1F
	Comentari string // // això
	Funcio    string // nom de funció i de mètode
	Tipus     string // int, string, noms de tipus i classes
	Operador  string // + - = { } ( )
	Nom       string // variables i la resta d'identificadors
}

// Tema és la paleta sencera d'un tema. Els valors són hex "#RRGGBB".
type Tema struct {
	Nom  string
	Clar bool

	// Superfícies, de la més enfonsada a la més elevada.
	Fons           string // fons de tot
	Abisme         string // barra lateral, panells, blocs de codi
	Night          string // superfície: targetes, entrada, menús
	SuperficieAlta string // superfície en hover o seleccionada

	// Text, del més fort al més apagat.
	Ink   string // títols, nom de la marca
	Text  string // text principal
	Slate string // metadades, descripcions
	Faint string // pistes de tecles, versió

	// Marca i accents.
	Escuma      string // primari
	EscumaClara string // primari en hover i crestes
	Arena       string // accent: avisos, mode CODE
	Onada       string // secundari: projecte, rols, hunks
	Algua       string // línies fines, rails, vores
	Lila        string // mode OBJECTIU (text)
	LilaFons    string // mode OBJECTIU (fons)
	Green       string // ok, afegit
	Red         string // error, esborrat

	// Vora del composer en repòs: l'algua aclarida. És l'únic to derivat
	// que es tolera (ve de docs/identitat.md).
	BoxIdle string

	Sintaxi Sintaxi
}

// Fosc és el tema de casa: grafit neutre amb l'escuma del Gregal.
func Fosc() Tema {
	return Tema{
		Nom:            "fosc",
		Fons:           "#171A19",
		Abisme:         "#101312",
		Night:          "#1E2321",
		SuperficieAlta: "#29302D",
		Ink:            "#F4F4EF",
		Text:           "#ECF0EA",
		Slate:          "#AEB9B1",
		Faint:          "#87978B",
		Escuma:         "#9FE0C4",
		EscumaClara:    "#BCEBD4",
		Arena:          "#DEC899",
		Onada:          "#8FB9A5",
		Algua:          "#264237",
		Lila:           "#C7B6D9",
		LilaFons:       "#30283A",
		Green:          "#7DE2A5",
		Red:            "#EF8D82",
		BoxIdle:        "#3C5750",
		Sintaxi: Sintaxi{
			Paraula:   "#C7B6D9", // lila
			Cadena:    "#9FE0C4", // escuma
			Numero:    "#DEC899", // arena
			Comentari: "#87978B",
			Funcio:    "#8FB9A5", // onada
			Tipus:     "#BCEBD4", // escuma clara
			Operador:  "#AEB9B1", // slate
			Nom:       "#ECF0EA", // text
		},
	}
}

// Clar és el mateix mar de dia. Els accents s'enfosqueixen: l'escuma i
// l'arena del fosc no passen AA sobre blanc.
func Clar() Tema {
	return Tema{
		Nom:            "clar",
		Clar:           true,
		Fons:           "#F5F7F8",
		Abisme:         "#EBEFF1",
		Night:          "#FFFFFF",
		SuperficieAlta: "#E2E9EC",
		Ink:            "#0B1220",
		Text:           "#1E293B",
		Slate:          "#52616B",
		// Faint puja de #7C8B95 a #5F6E78: el de abans donava 3,2:1 sobre
		// el panell clar i no passava AA per a text petit.
		Faint:       "#5F6E78",
		Escuma:      "#0E8C74",
		EscumaClara: "#0A6F5C",
		Arena:       "#9A6B12",
		Onada:       "#0E7490",
		Algua:       "#CBD5DC",
		Lila:        "#6D28D9",
		LilaFons:    "#EDE9FE",
		Green:       "#15803D",
		Red:         "#DC2626",
		BoxIdle:     "#7C8B95",
		Sintaxi: Sintaxi{
			Paraula:   "#6D28D9", // lila clar
			Cadena:    "#0A6F5C", // escuma clara (fosca, sobre blanc)
			Numero:    "#9A6B12", // arena clar
			Comentari: "#6B7A85",
			Funcio:    "#0E7490", // onada clar
			Tipus:     "#0E8C74", // escuma clar
			Operador:  "#52616B", // slate clar
			Nom:       "#1E293B", // text clar
		},
	}
}

// Gpt és un clar neutre inspirat en ChatGPT: blanc i grisos càlids, amb el
// verd de marca com a accent. Els grisos passen AA sobre blanc.
func Gpt() Tema {
	return Tema{
		Nom:            "gpt",
		Clar:           true,
		Fons:           "#FFFFFF",
		Abisme:         "#F9F9F9",
		Night:          "#FFFFFF",
		SuperficieAlta: "#ECECEC",
		Ink:            "#0D0D0D",
		Text:           "#1A1A1A",
		Slate:          "#5D5D5D",
		Faint:          "#707070",
		Escuma:         "#10A37F",
		EscumaClara:    "#0B7A5F",
		Arena:          "#B45309",
		Onada:          "#0E7490",
		Algua:          "#E5E5E5",
		Lila:           "#7C3AED",
		LilaFons:       "#EFE9FB",
		Green:          "#15803D",
		Red:            "#DC2626",
		BoxIdle:        "#B4B4B4",
		Sintaxi: Sintaxi{
			Paraula:   "#7C3AED",
			Cadena:    "#0B7A5F",
			Numero:    "#B45309",
			Comentari: "#707070",
			Funcio:    "#0E7490",
			Tipus:     "#10A37F",
			Operador:  "#5D5D5D",
			Nom:       "#1A1A1A",
		},
	}
}

// Claude és un crema càlid inspirat en Claude: fons color closca, text
// tinta gairebé negra i corall com a accent. Tot el text passa AA.
func Claude() Tema {
	return Tema{
		Nom:            "claude",
		Clar:           true,
		Fons:           "#F0EEE6",
		Abisme:         "#E7E2D3",
		Night:          "#FAF9F4",
		SuperficieAlta: "#DED8C4",
		Ink:            "#1F1E1D",
		Text:           "#262522",
		Slate:          "#5C5852",
		Faint:          "#6E6961",
		Escuma:         "#C15F3C",
		EscumaClara:    "#A34A2B",
		Arena:          "#96711B",
		Onada:          "#58746B",
		Algua:          "#D5CFC0",
		Lila:           "#6D4FA8",
		LilaFons:       "#E7DFD3",
		Green:          "#2E7D4F",
		Red:            "#C13B2E",
		BoxIdle:        "#A09A8C",
		Sintaxi: Sintaxi{
			Paraula:   "#6D4FA8",
			Cadena:    "#A34A2B",
			Numero:    "#96711B",
			Comentari: "#8A8478",
			Funcio:    "#58746B",
			Tipus:     "#C15F3C",
			Operador:  "#5C5852",
			Nom:       "#262522",
		},
	}
}

// Opencode és un fosc neutre inspirat en OpenCode: negre de zinc amb un
// taronja contingut com a accent. Tema fosc: el faint és decoratiu.
func Opencode() Tema {
	return Tema{
		Nom:            "opencode",
		Clar:           false,
		Fons:           "#0A0A0C",
		Abisme:         "#121216",
		Night:          "#17171C",
		SuperficieAlta: "#232229",
		Ink:            "#FAFAFA",
		Text:           "#E4E4E7",
		Slate:          "#A1A1AA",
		Faint:          "#71717A",
		Escuma:         "#E8833A",
		EscumaClara:    "#F0955A",
		Arena:          "#D9A441",
		Onada:          "#6FC7BD",
		Algua:          "#26262C",
		Lila:           "#B49DF5",
		LilaFons:       "#221E38",
		Green:          "#4ADE80",
		Red:            "#F97066",
		BoxIdle:        "#3A3A44",
		Sintaxi: Sintaxi{
			Paraula:   "#B49DF5",
			Cadena:    "#E8833A",
			Numero:    "#D9A441",
			Comentari: "#5B5B66",
			Funcio:    "#6FC7BD",
			Tipus:     "#F0955A",
			Operador:  "#A1A1AA",
			Nom:       "#E4E4E7",
		},
	}
}

// Mistral és un clar càlid inspirat en Le Chat: blanc trencat amb el
// taronja de marca (enfosquit de #FF7000 a #CE4E00: el pur donava 2,7:1
// sobre blanc i no es llegia ni en botons).
func Mistral() Tema {
	return Tema{
		Nom:            "mistral",
		Clar:           true,
		Fons:           "#FFFBF4",
		Abisme:         "#F5EEE2",
		Night:          "#FFFFFF",
		SuperficieAlta: "#EADFCB",
		Ink:            "#1A140D",
		Text:           "#2A231A",
		Slate:          "#6B5F4E",
		Faint:          "#7D7264",
		Escuma:         "#CE4E00",
		EscumaClara:    "#A63D00",
		Arena:          "#A86A08",
		Onada:          "#14707E",
		Algua:          "#E3D6C2",
		Lila:           "#7C3AED",
		LilaFons:       "#F1E8F8",
		Green:          "#15803D",
		Red:            "#DC2626",
		BoxIdle:        "#B8A88F",
		Sintaxi: Sintaxi{
			Paraula:   "#7C3AED",
			Cadena:    "#A63D00",
			Numero:    "#A86A08",
			Comentari: "#7D7264",
			Funcio:    "#14707E",
			Tipus:     "#CE4E00",
			Operador:  "#6B5F4E",
			Nom:       "#2A231A",
		},
	}
}

// Gemini és un clar blavós inspirat en Gemini: blanc amb un punt de blau
// i el blau de marca com a accent.
func Gemini() Tema {
	return Tema{
		Nom:            "gemini",
		Clar:           true,
		Fons:           "#F8FAFF",
		Abisme:         "#E9EFFA",
		Night:          "#FFFFFF",
		SuperficieAlta: "#D8E2F5",
		Ink:            "#0B1B33",
		Text:           "#17294D",
		Slate:          "#4E5E7A",
		Faint:          "#5F6E85",
		Escuma:         "#1A73E8",
		EscumaClara:    "#0B57D0",
		Arena:          "#B45309",
		Onada:          "#9333EA",
		Algua:          "#C9D7EE",
		Lila:           "#6D28D9",
		LilaFons:       "#ECE5FA",
		Green:          "#15803D",
		Red:            "#DC2626",
		BoxIdle:        "#9FB2D2",
		Sintaxi: Sintaxi{
			Paraula:   "#6D28D9",
			Cadena:    "#0B57D0",
			Numero:    "#B45309",
			Comentari: "#5F6E85",
			Funcio:    "#9333EA",
			Tipus:     "#1A73E8",
			Operador:  "#4E5E7A",
			Nom:       "#17294D",
		},
	}
}

// Grok és un fosc mínim inspirat en Grok: negre pur i monocrom, amb el
// blanc com a accent. Els colors funcionals (ok, error, avisos) es
// mantenen: un tema sense vermell no avisa de res.
func Grok() Tema {
	return Tema{
		Nom:            "grok",
		Clar:           false,
		Fons:           "#000000",
		Abisme:         "#0D0D0F",
		Night:          "#161618",
		SuperficieAlta: "#26262A",
		Ink:            "#FFFFFF",
		Text:           "#EDEDEF",
		Slate:          "#A8A8B0",
		Faint:          "#7E7E87",
		Escuma:         "#F5F5F5",
		EscumaClara:    "#B0B0B8",
		Arena:          "#D9A441",
		Onada:          "#8E8E93",
		Algua:          "#232326",
		Lila:           "#C4B5FD",
		LilaFons:       "#221E38",
		Green:          "#4ADE80",
		Red:            "#F97066",
		BoxIdle:        "#3A3A40",
		Sintaxi: Sintaxi{
			Paraula:   "#C4B5FD",
			Cadena:    "#EDEDEF",
			Numero:    "#D9A441",
			Comentari: "#5B5B60",
			Funcio:    "#8E8E93",
			Tipus:     "#F5F5F5",
			Operador:  "#A8A8B0",
			Nom:       "#EDEDEF",
		},
	}
}

// Descripcio és la línia d'un tema al menú del TUI.
func Descripcio(nom string) string {
	switch nom {
	case "fosc":
		return "fosc  (la marea: nit, escuma i arena)"
	case "clar":
		return "clar  (dia d'estiu: claror mediterrània)"
	case "gpt":
		return "gpt  (blanc i verd, aire ChatGPT)"
	case "claude":
		return "claude  (closca i corall, aire Claude)"
	case "opencode":
		return "opencode  (zinc i taronja, aire OpenCode)"
	case "mistral":
		return "mistral  (blanc càlid i taronja, aire Le Chat)"
	case "gemini":
		return "gemini  (blanc blavós i blau, aire Gemini)"
	case "grok":
		return "grok  (negre pur i blanc, aire Grok)"
	}
	return nom
}

// Tots són els temes en l'ordre en què es generen al CSS i es mostren
// als menús: primer els de casa, després els inspirats.
func Tots() []Tema {
	return []Tema{Fosc(), Clar(), Gpt(), Claude(), Opencode(), Mistral(), Gemini(), Grok()}
}

// Per retorna el tema amb aquest nom (fosc si no el coneix).
func Per(nom string) Tema {
	switch nom {
	case "clar", "light":
		return Clar()
	case "gpt":
		return Gpt()
	case "claude":
		return Claude()
	case "opencode":
		return Opencode()
	case "mistral":
		return Mistral()
	case "gemini":
		return Gemini()
	case "grok":
		return Grok()
	}
	return Fosc()
}

// Noms són els temes disponibles.
func Noms() []string {
	noms := make([]string, 0, 8)
	for _, t := range Tots() {
		noms = append(noms, t.Nom)
	}
	return noms
}
