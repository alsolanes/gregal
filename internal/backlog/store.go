package backlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/profile"
)

// Store és el backlog d'un projecte, sigui on sigui que visqui.
type Store struct {
	Root    string `json:"root"`
	Storage string `json:"storage"`
	// Path és el fitxer JSON (app) o la carpeta .gregal/backlog (repo).
	Path string `json:"path"`
}

// RepoDir és on viuen les tasques d'un projecte que les vol al repositori.
func RepoDir(root string) string { return filepath.Join(root, ".gregal", "backlog") }

// AppFile és on viu el backlog d'un projecte a les dades de l'app.
func AppFile(root string) string {
	h := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(root))))
	nom := slug(filepath.Base(root))
	if nom == "" {
		nom = "projecte"
	}
	return filepath.Join(profile.DataDir(), "backlog", nom+"-"+hex.EncodeToString(h[:4])+".json")
}

// Open troba el backlog del projecte on és dir.
func Open(dir string) (*Store, error) {
	root := Root(dir)
	if root == "" {
		return nil, ErrNoDir
	}
	if fi, err := os.Stat(RepoDir(root)); err == nil && fi.IsDir() {
		return &Store{Root: root, Storage: StorageRepo, Path: RepoDir(root)}, nil
	}
	return &Store{Root: root, Storage: StorageApp, Path: AppFile(root)}, nil
}

// Un mutex per arrel: el servidor web, l'eina i els vents poden escriure
// alhora el mateix backlog.
var locks sync.Map

func (s *Store) lock() func() {
	m, _ := locks.LoadOrStore(strings.ToLower(s.Root), &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

type appDoc struct {
	Project string `json:"project"`
	Items   []Item `json:"items"`
}

// List torna totes les tasques, per número.
func (s *Store) List() ([]Item, error) {
	defer s.lock()()
	return s.read()
}

func (s *Store) read() ([]Item, error) {
	var items []Item
	if s.Storage == StorageRepo {
		rutes, err := filepath.Glob(filepath.Join(s.Path, "*.md"))
		if err != nil {
			return nil, err
		}
		for _, r := range rutes {
			raw, err := os.ReadFile(r)
			if err != nil {
				return nil, err
			}
			it, err := ParseMarkdown(string(raw))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Base(r), err)
			}
			items = append(items, it)
		}
	} else {
		raw, err := os.ReadFile(s.Path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if len(raw) > 0 {
			var doc appDoc
			if err := json.Unmarshal(raw, &doc); err != nil {
				return nil, fmt.Errorf("%s: %w", s.Path, err)
			}
			items = doc.Items
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

// write desa les tasques canviades (repo) o el document sencer (app).
func (s *Store) write(all []Item, changed ...Item) error {
	if s.Storage == StorageRepo {
		if err := os.MkdirAll(s.Path, 0o755); err != nil {
			return err
		}
		for _, it := range changed {
			if err := writeAtomic(filepath.Join(s.Path, strconv.Itoa(it.ID)+".md"), []byte(Markdown(it)), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(appDoc{Project: s.Root, Items: all}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.Path, raw, 0o600)
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func find(all []Item, id int) int {
	for i := range all {
		if all[i].ID == id {
			return i
		}
	}
	return -1
}

// Get torna una tasca.
func (s *Store) Get(id int) (Item, error) {
	all, err := s.List()
	if err != nil {
		return Item{}, err
	}
	if i := find(all, id); i >= 0 {
		return all[i], nil
	}
	return Item{}, ErrNotFound
}

// Create afegeix una tasca nova amb el número següent.
func (s *Store) Create(it Item, who string) (Item, error) {
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	if err := normalitza(&it); err != nil {
		return Item{}, err
	}
	if it.Status != StatusProposed {
		// Una tasca nova sempre comença proposada: ready vol dir que algú
		// n'ha confirmat el perquè i els criteris, i això es fa movent-la.
		it.Status = StatusProposed
	}
	if err := checkRefs(it, all); err != nil {
		return Item{}, err
	}
	it.ID = 0
	for _, f := range all {
		if f.ID >= it.ID {
			it.ID = f.ID + 1
		}
	}
	if it.ID == 0 {
		it.ID = 1
	}
	now := time.Now().UTC()
	it.Created, it.Updated = now, now
	it.Log = []LogEntry{{At: now, Who: who, Text: "created"}}
	all = append(all, it)
	return it, s.write(all, it)
}

func checkRefs(it Item, all []Item) error {
	if it.Parent != 0 {
		i := find(all, it.Parent)
		if i < 0 {
			return fmt.Errorf("the parent #%d does not exist", it.Parent)
		}
		if all[i].Type != TypeEpic {
			return fmt.Errorf("the parent #%d is not an epic", it.Parent)
		}
	}
	for _, d := range it.DependsOn {
		if d == it.ID && it.ID != 0 {
			return errors.New("an item cannot depend on itself")
		}
		if find(all, d) < 0 {
			return fmt.Errorf("dependency #%d does not exist", d)
		}
	}
	return nil
}

// Edit canvia els camps d'una tasca (no l'estat ni el log: per això hi ha
// Move i AddLog). Canviar criteris d'una tasca en curs queda al log.
func (s *Store) Edit(id int, who string, fn func(*Item)) (Item, error) {
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	i := find(all, id)
	if i < 0 {
		return Item{}, ErrNotFound
	}
	it := all[i]
	abans := it
	fn(&it)
	it.ID, it.Status, it.Log, it.Created = abans.ID, abans.Status, abans.Log, abans.Created
	if err := normalitza(&it); err != nil {
		return Item{}, err
	}
	if err := checkRefs(it, all); err != nil {
		return Item{}, err
	}
	if canviats := campsCanviats(abans, it); canviats != "" {
		it.Updated = time.Now().UTC()
		it.Log = append(append([]LogEntry(nil), it.Log...), LogEntry{At: it.Updated, Who: who, Text: "edited " + canviats})
	}
	all[i] = it
	return it, s.write(all, it)
}

func campsCanviats(a, b Item) string {
	var c []string
	if a.Title != b.Title {
		c = append(c, "title")
	}
	if a.Type != b.Type {
		c = append(c, "type")
	}
	if a.Priority != b.Priority {
		c = append(c, "priority")
	}
	if a.Size != b.Size {
		c = append(c, "size")
	}
	if a.Why != b.Why {
		c = append(c, "why")
	}
	if a.OutOfScope != b.OutOfScope {
		c = append(c, "out of scope")
	}
	if a.Parent != b.Parent || fmt.Sprint(a.DependsOn) != fmt.Sprint(b.DependsOn) {
		c = append(c, "links")
	}
	ca, _ := json.Marshal(a.Criteria)
	cb, _ := json.Marshal(b.Criteria)
	if string(ca) != string(cb) {
		c = append(c, "criteria")
	}
	return strings.Join(c, ", ")
}

// Move canvia l'estat seguint CheckMove i ho apunta al log.
func (s *Store) Move(id int, to, reason, who string) (Item, error) {
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	i := find(all, id)
	if i < 0 {
		return Item{}, ErrNotFound
	}
	to = normalStatus(to)
	it := all[i]
	if err := CheckMove(it, to, reason, all); err != nil {
		return Item{}, err
	}
	reason = strings.TrimSpace(reason)
	text := it.Status + " → " + to
	if reason != "" {
		text += ": " + reason
	}
	it.Status = to
	if to == StatusBlocked || to == StatusDropped {
		it.Reason = reason
	} else {
		it.Reason = ""
	}
	if to == StatusInReview {
		// Una revisió vella (p. ex. la que la va tornar enrere) ja no diu
		// res del canvi d'ara.
		it.Review = nil
	}
	it.Updated = time.Now().UTC()
	it.Log = append(it.Log, LogEntry{At: it.Updated, Who: who, Text: text})
	all[i] = it
	if err := s.write(all, it); err != nil {
		return Item{}, err
	}
	if to == StatusInReview && OnInReview != nil {
		go OnInReview(s.Root, it)
	}
	return it, nil
}

// SetWork apunta on s'hi treballa i des de quin commit. Els buits no
// esborren el que hi havia.
func (s *Store) SetWork(id int, workdir, base string) (Item, error) {
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	i := find(all, id)
	if i < 0 {
		return Item{}, ErrNotFound
	}
	if workdir = strings.TrimSpace(workdir); workdir != "" {
		all[i].Workdir = workdir
	}
	if base = strings.TrimSpace(base); base != "" {
		all[i].Base = base
	}
	return all[i], s.write(all, all[i])
}

// SetReview desa una revisió i n'aplica el resultat:
//   - running: només ho apunta;
//   - met: la tasca queda a in_review, a punt perquè la persona la tanqui;
//   - not_met: torna a in_progress amb el que falta com a motiu, i els
//     criteris que el revisor no ha vist complerts es desmarquen;
//   - error: queda a in_review i decideix la persona.
func (s *Store) SetReview(id int, r Review, who string) (Item, error) {
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	i := find(all, id)
	if i < 0 {
		return Item{}, ErrNotFound
	}
	it := all[i]
	if r.At.IsZero() {
		r.At = time.Now().UTC()
	}
	it.Review = &r
	it.Updated = r.At
	logl := func(t string) { it.Log = append(it.Log, LogEntry{At: r.At, Who: who, Text: t}) }
	switch r.Status {
	case ReviewRunning:
		logl("review started")
	case ReviewMet:
		logl("review: MET" + resumeix(r))
	case ReviewError:
		logl("review failed: " + r.Summary + "; the person decides")
	case ReviewNotMet:
		logl("review: NOT MET" + resumeix(r))
		// Els criteris del revisor van en el mateix ordre que els de la
		// tasca; si no en torna els mateixos, no se'n toca cap.
		if len(r.Criteria) == len(it.Criteria) {
			for n, c := range r.Criteria {
				if c.Status != "MET" && it.Criteria[n].Done {
					it.Criteria[n].Done, it.Criteria[n].Evidence = false, ""
					logl(fmt.Sprintf("criterion %d unticked by the review", n+1))
				}
			}
		}
		if it.Status == StatusInReview {
			motiu := strings.TrimSpace(r.Summary)
			if motiu == "" {
				motiu = "see the review"
			}
			it.Status = StatusInProgress
			logl("in_review → in_progress: " + motiu)
		}
	default:
		return Item{}, fmt.Errorf("unknown review status %q", r.Status)
	}
	all[i] = it
	return it, s.write(all, it)
}

func resumeix(r Review) string {
	var parts []string
	for _, c := range r.Checks {
		estat := "pass"
		if !c.Passed {
			estat = "FAIL"
		}
		parts = append(parts, "`"+c.Command+"` "+estat)
	}
	for n, c := range r.Criteria {
		t := fmt.Sprintf("%d %s", n+1, c.Status)
		if c.Evidence != "" {
			t += " (" + c.Evidence + ")"
		}
		parts = append(parts, t)
	}
	if r.Summary != "" && r.Status != ReviewNotMet {
		parts = append(parts, r.Summary)
	}
	if len(parts) == 0 {
		return ""
	}
	return ": " + strings.Join(parts, "; ")
}

// Check marca (o desmarca, amb done=false) un criteri, 1-indexat. Marcar-lo
// demana la prova.
func (s *Store) Check(id, n int, done bool, evidence, who string) (Item, error) {
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	i := find(all, id)
	if i < 0 {
		return Item{}, ErrNotFound
	}
	it := all[i]
	if n < 1 || n > len(it.Criteria) {
		return Item{}, fmt.Errorf("item #%d has %d criteria; there is no criterion %d", id, len(it.Criteria), n)
	}
	evidence = strings.TrimSpace(evidence)
	if done && evidence == "" {
		return Item{}, errors.New("a criterion is only ticked with its evidence (test, command output, file, screenshot)")
	}
	c := &it.Criteria[n-1]
	c.Done = done
	if done {
		c.Evidence = evidence
	} else {
		c.Evidence = ""
	}
	it.Updated = time.Now().UTC()
	text := fmt.Sprintf("criterion %d ticked: %s", n, evidence)
	if !done {
		text = fmt.Sprintf("criterion %d unticked", n)
	}
	it.Log = append(it.Log, LogEntry{At: it.Updated, Who: who, Text: text})
	all[i] = it
	return it, s.write(all, it)
}

// AddLog afegeix una línia al log.
func (s *Store) AddLog(id int, text, who string) (Item, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Item{}, errors.New("empty log line")
	}
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return Item{}, err
	}
	i := find(all, id)
	if i < 0 {
		return Item{}, ErrNotFound
	}
	it := all[i]
	it.Updated = time.Now().UTC()
	it.Log = append(it.Log, LogEntry{At: it.Updated, Who: who, Text: text})
	all[i] = it
	return it, s.write(all, it)
}

// MoveToRepo passa el backlog de les dades de l'app a .gregal/backlog/ del
// projecte. El JSON antic queda amb l'extensió .moved, per si de cas.
func (s *Store) MoveToRepo() error {
	if s.Storage == StorageRepo {
		return nil
	}
	defer s.lock()()
	all, err := s.read()
	if err != nil {
		return err
	}
	repo := &Store{Root: s.Root, Storage: StorageRepo, Path: RepoDir(s.Root)}
	if err := repo.write(all, all...); err != nil {
		return err
	}
	if err := os.MkdirAll(repo.Path, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(s.Path); err == nil {
		if err := os.Rename(s.Path, s.Path+".moved"); err != nil {
			return err
		}
	}
	*s = *repo
	return nil
}

var noSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = noSlug.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	return s
}

// Slug fa servir el títol per anomenar branques i worktrees.
func Slug(it Item) string {
	s := slug(it.Title)
	if len(s) > 24 {
		s = strings.Trim(s[:24], "-")
	}
	if s == "" {
		return "b" + strconv.Itoa(it.ID)
	}
	return "b" + strconv.Itoa(it.ID) + "-" + s
}
