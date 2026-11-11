// Package connexions és el model únic de connexions de cada persona (pla v2
// §6, paquet B5): connectors OAuth/API, calendaris ICS, correu IMAP i
// sessions al navegador de l'ordinador, amb un estat comprovat de debò i una
// referència opaca (authRef «auth:…») al secret.
//
// Els secrets (contrasenyes d'aplicació, adreces secretes iCal, tokens OAuth)
// no viuen mai al registre ni a cap payload: són al magatzem xifrat d'aquest
// fitxer, un per owner, sota el directori de dades.
package connexions

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/profile"
)

// ErrNoSecret: la referència no té cap secret desat (desconnectat o mai
// connectat).
var ErrNoSecret = errors.New("connexions: no secret for this reference")

// Vault és el magatzem de secrets xifrat amb AES-256-GCM.
//
//	<dir>/vault.key          clau local de 32 bytes, generada la primera vegada (0600)
//	<dir>/<owner>.vault.json un fitxer per owner (0600): ref → {nonce, xifrat}
//
// La clau viatja amb el directori de dades (la còpia de seguretat la porta):
// sense ella els secrets no es poden llegir. Les dades addicionals
// autenticades són owner+ref: un xifrat copiat al fitxer d'un altre owner o a
// una altra referència no es desxifra.
type Vault struct {
	dir string
	mu  sync.Mutex
	key []byte
}

var (
	defaultMu    sync.Mutex
	defaultVault = map[string]*Vault{}
)

// VaultDir és on viu el magatzem per defecte: <dades de Gregal>/secrets.
func VaultDir() string { return filepath.Join(profile.DataDir(), "secrets") }

// DefaultVault és el magatzem del directori de dades actual (un per camí:
// els tests canvien GREGAL_DATA_DIR).
func DefaultVault() *Vault {
	dir := VaultDir()
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if v := defaultVault[dir]; v != nil {
		return v
	}
	v := OpenVault(dir)
	defaultVault[dir] = v
	return v
}

// OpenVault obre (sense crear-lo encara) el magatzem de dir.
func OpenVault(dir string) *Vault { return &Vault{dir: dir} }

// NewAuthRef és una referència opaca nova: «auth:» + 20 hex aleatoris.
func NewAuthRef() string { return "auth:" + randomHex(10) }

// NewID és un identificador de connexió nou: «con-» + 16 hex aleatoris.
func NewID() string { return "con-" + randomHex(8) }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func ownerFile(owner string) string {
	if owner == "" {
		return "local.vault.json"
	}
	sum := sha256.Sum256([]byte("gregal-owner\x00" + owner))
	return "o-" + hex.EncodeToString(sum[:12]) + ".vault.json"
}

func (v *Vault) loadKey() ([]byte, error) {
	if v.key != nil {
		return v.key, nil
	}
	path := filepath.Join(v.dir, "vault.key")
	if raw, err := os.ReadFile(path); err == nil {
		k, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if derr != nil || len(k) != 32 {
			// Mai se sobreescriu una clau il·legible: es perdrien tots els secrets.
			return nil, errors.New("connexions: the vault key is unreadable; restore it from the backup")
		}
		v.key = k
		return k, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(v.dir, 0o700); err != nil {
		return nil, err
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	// O_EXCL: si dos processos la creen alhora, guanya el primer i l'altre la llegeix.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return v.loadKey()
		}
		return nil, err
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(k) + "\n"); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	v.key = k
	return k, nil
}

type sealed struct {
	Nonce string `json:"n"`
	Data  string `json:"c"`
}

type vaultFile struct {
	V     int               `json:"v"`
	Items map[string]sealed `json:"items"`
}

func (v *Vault) path(owner string) string { return filepath.Join(v.dir, ownerFile(owner)) }

func (v *Vault) read(owner string) (vaultFile, error) {
	f := vaultFile{V: 1, Items: map[string]sealed{}}
	raw, err := os.ReadFile(v.path(owner))
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, fmt.Errorf("connexions: unreadable vault: %w", err)
	}
	if f.Items == nil {
		f.Items = map[string]sealed{}
	}
	return f, nil
}

func (v *Vault) write(owner string, f vaultFile) error {
	if err := os.MkdirAll(v.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	return writeAtomic(v.path(owner), raw)
}

func (v *Vault) aead() (cipher.AEAD, error) {
	k, err := v.loadKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func aad(owner, ref string) []byte { return []byte("gregal-vault-v1\x00" + owner + "\x00" + ref) }

// Put desa (o substitueix) el secret de ref per a owner.
func (v *Vault) Put(owner, ref string, secret []byte) error {
	if ref == "" {
		return errors.New("connexions: empty reference")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	g, err := v.aead()
	if err != nil {
		return err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	f, err := v.read(owner)
	if err != nil {
		return err
	}
	f.Items[ref] = sealed{
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(g.Seal(nil, nonce, secret, aad(owner, ref))),
	}
	return v.write(owner, f)
}

// Get torna el secret de ref per a owner (ErrNoSecret si no n'hi ha).
func (v *Vault) Get(owner, ref string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	f, err := v.read(owner)
	if err != nil {
		return nil, err
	}
	s, ok := f.Items[ref]
	if !ok {
		return nil, ErrNoSecret
	}
	g, err := v.aead()
	if err != nil {
		return nil, err
	}
	return openSealed(g, s, owner, ref)
}

func openSealed(g cipher.AEAD, s sealed, owner, ref string) ([]byte, error) {
	nonce, err1 := base64.StdEncoding.DecodeString(s.Nonce)
	data, err2 := base64.StdEncoding.DecodeString(s.Data)
	if err1 != nil || err2 != nil || len(nonce) != g.NonceSize() {
		return nil, errors.New("connexions: damaged secret")
	}
	out, err := g.Open(nil, nonce, data, aad(owner, ref))
	if err != nil {
		return nil, errors.New("connexions: the secret cannot be decrypted with this vault key")
	}
	return out, nil
}

// Delete esborra el secret de ref (no és error si no n'hi havia).
func (v *Vault) Delete(owner, ref string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	f, err := v.read(owner)
	if err != nil {
		return err
	}
	if _, ok := f.Items[ref]; !ok {
		return nil
	}
	delete(f.Items, ref)
	return v.write(owner, f)
}

// PutJSON i GetJSON desen estructures (tokens OAuth) com a secret.
func (v *Vault) PutJSON(owner, ref string, x any) error {
	raw, err := json.Marshal(x)
	if err != nil {
		return err
	}
	return v.Put(owner, ref, raw)
}

func (v *Vault) GetJSON(owner, ref string, x any) error {
	raw, err := v.Get(owner, ref)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, x)
}

func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Chmod(f.Name(), 0o600)
	err = os.Rename(f.Name(), path)
	for i := 0; err != nil && i < 10; i++ {
		// Windows: el destí obert per un lector un instant.
		time.Sleep(15 * time.Millisecond)
		err = os.Rename(f.Name(), path)
	}
	return err
}
