// Package selfupdate actualitza Gregal des del seu propi repositori: baixa,
// compila, COMPROVA la compilació i només llavors substitueix el binari.
//
// Regles de la casa (perquè una actualització automàtica que va malament és
// pitjor que no tenir-ne):
//
//  1. Mai amb canvis sense cometre: es mira i s'atura amb un missatge clar.
//  2. Mai fusions: `git pull --ff-only`. Un actualitzador no ha de crear mai
//     un conflicte ni un commit de fusió que l'usuari no ha demanat.
//  3. El binari nou es comprova ABANS de substituir l'antic: si no arrenca o
//     no li agrada el config, l'antic es queda on era i es diu per què.
//  4. Substitucions atòmiques (rename dins el mateix disc) i còpia de
//     seguretat del binari anterior: tornar enrere és un sol mv.
//  5. Els serveis es reinicien i es COMPROVA que hagin quedat actius; si un
//     no ho està, es diu ben alt.
package selfupdate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Opcions és el que necessita una actualització.
type Opcions struct {
	Repo          string // camí del repositori (si és buit, es dedueix)
	Cwd           string // des d'on s'ha cridat (per deduir el repositori)
	Branca        string // branca a seguir (per defecte main)
	NomesComprova bool
	Forca         bool // recompila encara que estigui al dia
	Sortida       func(string)
	Error         func(string)
}

// Resultat explica què ha passat.
type Resultat struct {
	Repo          string
	AlDia         bool
	VersioAbans   string
	VersioDespres string
	Incidencies   []string
}

const (
	tempsMaxBaixada = 120 * time.Second
	tempsMaxBuild   = 300 * time.Second
)

// Executa fa l'actualització sencera. Els errors greus (no es pot compilar,
// el binari nou no arrenca) tornen com a error: l'antic segueix instal·lat.
func Executa(ctx context.Context, o Opcions) (*Resultat, error) {
	if o.Sortida == nil {
		o.Sortida = func(string) {}
	}
	if o.Error == nil {
		o.Error = o.Sortida
	}
	if o.Branca == "" {
		o.Branca = "main"
	}
	res := &Resultat{}

	repo := o.Repo
	if repo == "" {
		repo = DedueixRepo(o.Cwd)
	}
	if repo == "" {
		return res, fmt.Errorf("no trobo el repositori de Gregal; passa'l amb -repo o crida-ho des del projecte")
	}
	if fi, err := os.Stat(filepath.Join(repo, "go.mod")); err != nil || fi.IsDir() {
		return res, fmt.Errorf("%s no sembla el repositori de Gregal (no hi ha go.mod)", repo)
	}
	res.Repo = repo
	o.Sortida("📦 repositori: " + repo)

	// (1) Canvis sense cometre: aturar-se.
	brut, err := git(ctx, repo, "status", "--porcelain")
	if err != nil {
		return res, err
	}
	if strings.TrimSpace(brut) != "" && !o.Forca && !o.NomesComprova {
		return res, fmt.Errorf("hi ha canvis sense cometre a %s\n%s\nComete'ls o desa'ls (git stash) abans d'actualitzar", repo, indent(truncateLines(brut, 8)))
	}

	// (2) Baixar i mirar si hi ha res de nou.
	if _, err := git(ctx, repo, "fetch", "--prune", "origin", o.Branca); err != nil {
		return res, fmt.Errorf("no he pogut baixar del remot: %w", err)
	}
	local, err := git(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		return res, err
	}
	remot, err := git(ctx, repo, "rev-parse", "origin/"+o.Branca)
	if err != nil {
		return res, err
	}
	res.VersioAbans = versioDelBinari(o.Sortida)
	if local == remot {
		res.AlDia = true
		o.Sortida("✅ ja estàs al dia (" + short(local) + ", " + res.VersioAbans + ")")
		if !o.Forca {
			return res, nil
		}
		o.Sortida("↻ -forca: recompilo igualment")
	} else {
		darrere, _ := git(ctx, repo, "rev-list", "--count", local+".."+remot)
		o.Sortida(fmt.Sprintf("⬇️ hi ha %s commits nous (%s → %s)", strings.TrimSpace(darrere), short(local), short(remot)))
		if o.NomesComprova {
			return res, nil
		}
		// (2b) Només endavant: mai fusions.
		if out, err := git(ctx, repo, "merge", "--ff-only", "origin/"+o.Branca); err != nil {
			return res, fmt.Errorf("no puc avançar sense fusionar (has divergit): %w\n%s", err, truncateLines(out, 6))
		}
		o.Sortida("   arbre actualitzat")
	}

	// (3) Compilar a part (al mateix disc que el destí, perquè el rename
	// sigui atòmic) i comprovar el binari ABANS de tocar-ne cap.
	desti, err := camiBinari()
	if err != nil {
		return res, err
	}
	tmp, err := buildABaix(repo, desti)
	if err != nil {
		return res, err
	}
	defer os.Remove(tmp)
	o.Sortida("🔨 compilat: " + tmp)

	if out, err := exec.CommandContext(ctx, tmp, "--version").CombinedOutput(); err != nil {
		return res, fmt.Errorf("el binari nou no arrenca (%v): %s", err, strings.TrimSpace(string(out)))
	} else {
		res.VersioDespres = strings.TrimSpace(string(out))
	}
	// El config de debò: si el binari nou no l'accepta, no s'instal·la.
	cfg := filepath.Join(os.Getenv("HOME"), ".config", "gregal", "config.yaml")
	if _, err := os.Stat(cfg); err == nil {
		if out, err := exec.CommandContext(ctx, tmp, "--check-config", "--config", cfg).CombinedOutput(); err != nil {
			return res, fmt.Errorf("el binari nou no accepta el config actual (%v): %s", err, strings.TrimSpace(string(out)))
		}
		o.Sortida("   comprovat amb el config de debò")
	}

	// (4) Còpia de seguretat i substitució atòmica.
	anterior := desti + ".abans"
	if _, err := os.Stat(desti); err == nil {
		if err := copiar(desti, anterior); err != nil {
			return res, fmt.Errorf("no he pogut fer la còpia de seguretat: %w", err)
		}
	}
	if err := os.Rename(tmp, desti); err != nil {
		return res, fmt.Errorf("no he pogut instal·lar el binari nou: %w", err)
	}
	o.Sortida("✅ instal·lat a " + desti + " (l'anterior és a " + anterior + ")")

	// (5) Serveis: reiniciar els que hi ha i comprovar que quedin actius.
	for _, s := range serveisActius(ctx) {
		if err := reinicia(ctx, s); err != nil {
			res.Incidencies = append(res.Incidencies, fmt.Sprintf("%s no ha tornat: %v", s, err))
			continue
		}
		o.Sortida("🔄 " + s + " reiniciat")
	}
	return res, nil
}

// DedueixRepo puja des d'un directori buscant el go.mod de Gregal.
func DedueixRepo(desde string) string {
	if desde == "" {
		desde, _ = os.Getwd()
	}
	// L'aturada és «el pare és jo mateix», no comparar amb "/" o ".": a
	// Windows filepath.Dir("C:\\") torna "C:\\", i el bucle no s'acabava
	// mai. Des d'una carpeta fora del repo, `gregal update` es penjava
	// llegint C:\go.mod per sempre (i la suite de tests hi moria als deu
	// minuts).
	for d := desde; d != ""; {
		b, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil && strings.Contains(string(b), "module gregal") {
			return d
		}
		pare := filepath.Dir(d)
		if pare == d {
			break
		}
		d = pare
	}
	return ""
}

// buildABaix compila en un fitxer temporal del mateix disc que el destí (el
// rename entre discs no és atòmic, i una actualització a mitges trencaria el
// binari).
func buildABaix(repo, desti string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(desti), 0o755); err != nil {
		return "", err
	}
	tmp := filepath.Join(filepath.Dir(desti), fmt.Sprintf(".gregal-update-%d", os.Getpid()))
	ctx, cancel := context.WithTimeout(context.Background(), tempsMaxBuild)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBinari(), "build", "-o", tmp,
		"-ldflags", "-X main.repoDir="+repo, ".")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("no s'ha pogut compilar:\n%s", truncateLines(string(out), 20))
	}
	return tmp, nil
}

// goBinari troba la cadena d'eines: primer el PATH i, si no, el lloc on viu
// el Go d'aquesta màquina.
func goBinari() string {
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	casa, _ := os.UserHomeDir()
	alt := filepath.Join(casa, "sdk", "go", "bin", "go")
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return "go"
}

// camiBinari és on viu el binari que està corrent: s'actualitza ell mateix
// on és, sense suposar cap ruta.
func camiBinari() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("no sé on sóc: %w", err)
	}
	if resolt, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolt
	}
	return exe, nil
}

// serveisActius llista les unitats d'usuari de Gregal que estan actives.
func serveisActius(ctx context.Context) []string {
	out, err := exec.CommandContext(ctx, "systemctl", "--user", "list-units", "--type=service",
		"gregal-*", "--state=active", "--no-legend", "--plain").Output()
	if err != nil {
		return nil
	}
	var s []string
	for _, linia := range strings.Split(string(out), "\n") {
		camps := strings.Fields(linia)
		if len(camps) > 0 && strings.HasSuffix(camps[0], ".service") {
			s = append(s, campos(camps[0]))
		}
	}
	return s
}

// campos treu el sufix .service per passar-ho a systemctl.
func campos(u string) string { return strings.TrimSuffix(u, ".service") }

// reinicia reinicia un servei i comprova que quedi actiu.
func reinicia(ctx context.Context, unitat string) error {
	if out, err := exec.CommandContext(ctx, "systemctl", "--user", "restart", unitat).CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	time.Sleep(2 * time.Second)
	if err := exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", unitat).Run(); err != nil {
		return fmt.Errorf("no ha quedat actiu")
	}
	return nil
}

// versioDelBinari diu quina versió corre ara (pel binari instal·lat, no pel
// codi: són coses diferents i l'usuari vol saber la segona).
func versioDelBinari(sortida func(string)) string {
	exe, err := camiBinari()
	if err != nil {
		return "desconeguda"
	}
	out, err := exec.Command(exe, "--version").Output()
	if err != nil {
		return "desconeguda"
	}
	return strings.TrimSpace(string(out))
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(ctx, tempsMaxBaixada)
	defer cancel()
	cmd := exec.CommandContext(c, "git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func copiar(des, cap string) error {
	b, err := os.ReadFile(des)
	if err != nil {
		return err
	}
	return os.WriteFile(cap, b, 0o755)
}

func short(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func truncateLines(s string, n int) string {
	linies := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(linies) > n {
		linies = append(linies[:n], fmt.Sprintf("… (%d línies més)", len(linies)-n))
	}
	return strings.Join(linies, "\n")
}

func indent(s string) string {
	linies := strings.Split(s, "\n")
	for i, l := range linies {
		linies[i] = "  " + l
	}
	return strings.Join(linies, "\n")
}
