// Package service descriu la instància local del servei Gregal.
//
// El descriptor és una pista de descobriment, no una autorització: el client
// sempre valida l'API, el protocol i la identitat amb /api/health abans d'usar
// l'adreça que hi troba.
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	Protocol       = "gregal.v1"
	DescriptorName = "service.json"
	LockName       = "service.lock"
)

// Descriptor és la informació mínima perquè un client local trobi el servei.
// El fitxer es desa amb permisos d'usuari i el token no s'inclou si el servei
// no ha estat configurat amb autenticació.
type Descriptor struct {
	Protocol  string    `json:"protocol"`
	Instance  string    `json:"instance_id"`
	Address   string    `json:"address"`
	Token     string    `json:"token,omitempty"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// DataDir comparteix la política de perfil amb sessions i la resta d'estat.
func DataDir() string {
	if dir := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal")
	}
	return filepath.Join(home, ".local", "share", "gregal")
}

func Path() string { return filepath.Join(DataDir(), DescriptorName) }

type Lease struct {
	file *os.File
	path string
	pid  int
}

// Acquire impedeix dues arrencades locals sobre el mateix perfil. El lock es
// manté fins a Release; una futura ordre de recuperació podrà netejar locks
// orfes després de validar el PID i l'estat del descriptor.
func Acquire() (*Lease, error) {
	dir := DataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, LockName)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return nil, err
			}
			return &Lease{file: f, path: path, pid: os.Getpid()}, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("servei ja en marxa o lock ocupat: %w", err)
		}
		pid, ok := lockPID(path)
		// Un lock buit pot pertànyer a una arrencada que encara escriu el PID;
		// no el toquem per evitar una carrera en aquesta finestra curta.
		if !ok || processAlive(pid) {
			return nil, fmt.Errorf("servei ja en marxa o lock ocupat: PID %d", pid)
		}
		// El rename és la recuperació atòmica: si dos processos observen el
		// mateix PID mort, només un pot moure aquest nom i l'altre torna a
		// intentar adquirir el lock nou.
		stale := fmt.Sprintf("%s.stale-%d-%d", path, os.Getpid(), attempt)
		if err := os.Rename(path, stale); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("no es pot recuperar el lock orfe: %w", err)
		}
		_ = os.Remove(stale)
	}
	return nil, fmt.Errorf("no es pot adquirir el lock del servei")
}

func lockPID(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	return pid, err == nil && pid > 0
}

func (l *Lease) Release() error {
	if l == nil {
		return nil
	}
	if l.file != nil {
		_ = l.file.Close()
	}
	if raw, err := os.ReadFile(l.path); err == nil && strings.TrimSpace(string(raw)) != fmt.Sprint(l.pid) {
		return nil
	}
	return os.Remove(l.path)
}

// Publish escriu el descriptor de manera atòmica i retorna una funció que
// només elimina el descriptor si encara apunta a aquesta mateixa instància.
func Publish(d Descriptor) (func(), error) {
	if strings.TrimSpace(d.Protocol) == "" || strings.TrimSpace(d.Instance) == "" || strings.TrimSpace(d.Address) == "" {
		return func() {}, errors.New("descriptor incomplet")
	}
	if d.StartedAt.IsZero() {
		d.StartedAt = time.Now().UTC()
	}
	dir := DataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return func() {}, err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return func() {}, err
	}
	tmp, err := os.CreateTemp(dir, ".service-*.tmp")
	if err != nil {
		return func() {}, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return func() {}, err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return func() {}, err
	}
	if err := tmp.Close(); err != nil {
		return func() {}, err
	}
	if err := os.Rename(tmpName, Path()); err != nil {
		// Windows no reemplaça un fitxer existent amb Rename. El lock del
		// servei serialitza aquest canvi de descriptor.
		_ = os.Remove(Path())
		if err2 := os.Rename(tmpName, Path()); err2 != nil {
			return func() {}, err2
		}
	}
	return func() { _ = Remove(d.Instance) }, nil
}

func Read() (Descriptor, error) {
	raw, err := os.ReadFile(Path())
	if err != nil {
		return Descriptor{}, err
	}
	var d Descriptor
	if err := json.Unmarshal(raw, &d); err != nil {
		return Descriptor{}, fmt.Errorf("descriptor corrupte: %w", err)
	}
	if d.Protocol == "" || d.Instance == "" || d.Address == "" {
		return Descriptor{}, errors.New("descriptor incomplet")
	}
	return d, nil
}

// Remove no elimina el descriptor d'un procés nou que hagi reutilitzat el
// mateix fitxer després d'un reinici.
func Remove(instance string) error {
	d, err := Read()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(instance) != "" && d.Instance != instance {
		return nil
	}
	return os.Remove(Path())
}
