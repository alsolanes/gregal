package web

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Productes de fora del paquet: Gregal Jocs (jocs/server) no viu a
// internal/web ni el binari de Gregal l'inclou. Un binari que el vol (el de
// jocs/cmd/gregal-jocs) l'importa i, des d'un init(), crida RegisterProduct
// abans d'arrencar el servidor. Aquest paquet no importa mai cap producte:
// només en recorre el registre.
//
// Un producte hi pot posar:
//   - una pàgina pròpia (Path), que és la web de sempre amb data-product=ID,
//     el seu <title> i el seu HTML a les marques <!--product:NOM--> d'index.html;
//   - plugins (rutes amb la mateixa porta i la mateixa autenticació que la
//     resta, bucles de fons Start/Stop i l'interruptor de /api/plugins);
//   - fitxers web propis servits a /app/ (els de la web base hi tenen
//     prioritat), així les URL no canvien en treure'ls d'aquí;
//   - convidats: peticions /api/ sense token que el producte deixa passar
//     (l'espectador de l'Arena, amb la clau de l'enllaç).
//
// Sense cap producte a Path, aquella pàgina explica que és una app a part.

// Product descriu un producte registrat des de fora del paquet.
type Product struct {
	// ID és el data-product de la pàgina ("jocs"); index.html hi tria les
	// vistes (PRODUCT_VIEWS).
	ID string
	// Path és on viu la pàgina del producte, sense barra final ("/jocs"):
	// respon Path i tot el que hi ha a sota.
	Path string
	// Title substitueix el <title> de la pàgina del producte (l'element
	// sencer, amb data-i18n si cal).
	Title string
	// Slots és l'HTML que va a cada marca <!--product:NOM--> d'index.html
	// (css, modules, end), només a la pàgina del producte. Les URL /app/…
	// hi porten la versió dels fitxers com la resta.
	Slots map[string]string
	// Assets són els fitxers web del producte: app/<nom> es serveix a
	// /app/<nom>. Opcional.
	Assets fs.FS
	// Plugins són les funcions del producte, com les de plugin_*.go.
	Plugins []*Plugin
	// Guest diu si una petició /api/ sense token és d'un convidat que el
	// producte accepta (es crida amb el servidor default). Les peticions
	// que passen per aquí no porten usuari i IsGuest hi diu true. Opcional.
	Guest func(s *Server, r *http.Request) bool
	// Routes són rutes del producte que no són de cap plugin: hi són
	// sempre, amb la mateixa autenticació que la resta (handle registra al
	// mux; Hub.Global lliga a la sessió de qui pregunta). Opcional.
	Routes func(h *Hub, handle func(string, http.HandlerFunc))
}

var (
	productMu  sync.Mutex
	productReg []*Product
)

// RegisterProduct afegeix un producte al registre (des d'un init(), abans
// de crear el servidor). Un ID repetit substitueix l'anterior.
func RegisterProduct(p *Product) {
	productMu.Lock()
	replaced := false
	for i, q := range productReg {
		if q.ID == p.ID {
			productReg[i], replaced = p, true
		}
	}
	if !replaced {
		productReg = append(productReg, p)
	}
	productMu.Unlock()
	for _, pl := range p.Plugins {
		pl.product = p.ID
		registerPlugin(pl)
	}
}

// noGregalBackground: aquest procés és el servidor d'un producte (Gregal
// Jocs) que pot córrer al costat del de Gregal sobre el mateix perfil.
var noGregalBackground atomic.Bool

// SetGregalBackground diu si aquest servidor fa la feina de fons de Gregal:
// les programacions (jobs i rutines dels vents), el bucle del Racó, la
// represa de les colles, les apps gestionades i els túnels. El binari de
// Gregal Jocs ho apaga (cli.Build.Product): corre al costat del de Gregal,
// sobre les mateixes dades, i amb dos programadors cada rutina sortiria dues
// vegades. Els plugins dels productes sí que arrenquen la seva.
func SetGregalBackground(on bool) { noGregalBackground.Store(!on) }

func gregalBackground() bool { return !noGregalBackground.Load() }

// RegisterPlugin afegeix un plugin solt al registre (des d'un init()). Els
// productes ja hi registren els seus amb RegisterProduct.
func RegisterPlugin(p *Plugin) { registerPlugin(p) }

func products() []*Product {
	productMu.Lock()
	defer productMu.Unlock()
	return append([]*Product(nil), productReg...)
}

// productAt retorna el producte que viu a path ("/jocs"), o nil.
func productAt(path string) *Product {
	for _, p := range products() {
		if p.Path == path {
			return p
		}
	}
	return nil
}

// ctxKeyGuest marca les peticions d'un convidat d'un producte.
type ctxKeyGuest struct{}

// IsGuest diu si la petició ha entrat com a convidat d'un producte (sense
// token, per Product.Guest).
func IsGuest(r *http.Request) bool {
	v, _ := r.Context().Value(ctxKeyGuest{}).(bool)
	return v
}

func productGuest(s *Server, r *http.Request) bool {
	for _, p := range products() {
		if p.Guest != nil && p.Guest(s, r) {
			return true
		}
	}
	return false
}

// productAsset busca app/<nom> als fitxers dels productes.
func productAsset(name string) ([]byte, error) {
	for _, p := range products() {
		if p.Assets == nil {
			continue
		}
		if raw, err := fs.ReadFile(p.Assets, name); err == nil {
			return raw, nil
		}
	}
	return nil, fs.ErrNotExist
}

// productsDigest resumeix el que els productes hi posen (HTML i fitxers):
// entra a la versió dels fitxers web, perquè un canvi en un producte també
// demani còpies noves al navegador.
func productsDigest() string {
	h := md5.New()
	for _, p := range products() {
		fmt.Fprintf(h, "%s|%s|%s|", p.ID, p.Path, p.Title)
		keys := make([]string, 0, len(p.Slots))
		for k := range p.Slots {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(h, "%s=%s|", k, p.Slots[k])
		}
		if p.Assets != nil {
			_ = fs.WalkDir(p.Assets, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				raw, _ := fs.ReadFile(p.Assets, path)
				fmt.Fprintf(h, "%s:%x|", path, md5.Sum(raw))
				return nil
			})
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// reProductSlot és una marca <!--product:NOM--> d'index.html.
var reProductSlot = regexp.MustCompile(`<!--product:([a-z]+)-->`)

// fillProductSlots posa a cada marca l'HTML del producte (p pot ser nil:
// llavors les marques queden buides).
func fillProductSlots(html string, p *Product) string {
	return reProductSlot.ReplaceAllStringFunc(html, func(m string) string {
		if p == nil {
			return ""
		}
		return p.Slots[reProductSlot.FindStringSubmatch(m)[1]]
	})
}

// productPage és la web de sempre com a pàgina del producte p.
func productPage(html string, p *Product) string {
	out := strings.Replace(html, `<html lang="ca">`, `<html lang="ca" data-product="`+p.ID+`">`, 1)
	if p.Title != "" {
		out = reTitle.ReplaceAllLiteralString(out, p.Title)
	}
	return fillProductSlots(out, p)
}

var reTitle = regexp.MustCompile(`<title[^>]*>[^<]*</title>`)

// JocsReleasesURL és on es baixa Gregal Jocs.
const JocsReleasesURL = "https://github.com/alsolanes/gregal/releases/latest"

// jocsApart és la pàgina /jocs/ d'un Gregal sense els jocs: ara són una app
// a part.
const jocsApart = `<!doctype html>
<html lang="ca">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Gregal Jocs</title>
<style>
body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;background:#0f1419;color:#e6e6e6;margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center}
main{max-width:34rem;padding:2rem;text-align:center;line-height:1.5}
h1{font-size:1.6rem;margin:0 0 .75rem}
p{margin:.5rem 0;color:#b8c0c8}
a.btn{display:inline-block;margin-top:1rem;padding:.6rem 1.1rem;border-radius:.5rem;background:#3b82f6;color:#fff;text-decoration:none}
a.back{display:block;margin-top:1rem;color:#8ab4f8}
</style>
</head>
<body>
<main>
<h1>🏆 Gregal Jocs</h1>
<p lang="en">The games (the Arena and the winds' league) are now a separate app, <b>Gregal Jocs</b>.</p>
<p lang="ca">Els jocs (l'Arena i la lliga dels vents) ara són una app a part, <b>Gregal Jocs</b>.</p>
<a class="btn" href="` + JocsReleasesURL + `" target="_blank" rel="noopener">Download Gregal Jocs · Baixa Gregal Jocs</a>
<a class="back" href="/">← Gregal</a>
</main>
</body>
</html>
`
