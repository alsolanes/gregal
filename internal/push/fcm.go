// Package push envia avisos de fons als mòbils amb Firebase Cloud Messaging
// (API HTTP v1). És opcional: sense credencials el servidor no envia res i
// l'app continua amb el sondeig (WindWatch), que és el mecanisme declarat
// per a entorns sense GMS.
//
// El payload d'un push només porta identificadors opacs: mai text privat.
// L'app, en rebre'l, llegeix el contingut amb la seva sessió autenticada.
package push

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/profile"
)

// EnvCredentials és el camí del fitxer JSON del compte de servei de
// Firebase. El fitxer viu fora del repositori; el seu contingut mai no es
// registra ni es torna per cap API.
const EnvCredentials = "GREGAL_FCM_CREDENTIALS"

// ErrUnregistered: el token ja no és vàlid (app desinstal·lada, token
// rotat). Qui envia l'ha d'oblidar.
var ErrUnregistered = errors.New("push: token no registrat")

// Sender envia un missatge de dades a un dispositiu.
type Sender interface {
	Send(ctx context.Context, token string, data map[string]string) error
}

type serviceAccount struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

// FCM és el Sender real (FCM HTTP v1 amb un compte de servei).
type FCM struct {
	acct   serviceAccount
	key    *rsa.PrivateKey
	client *http.Client
	// endpoint permet als tests apuntar a un servidor fals.
	endpoint string

	mu      sync.Mutex
	token   string
	expires time.Time
}

var (
	envOnce   sync.Once
	envSender Sender
	envErr    error
)

// FromEnv torna el Sender configurat o nil si no n'hi ha (sense error: és
// el mode «sondeig» declarat). Un fitxer present però il·legible és un error.
func FromEnv() (Sender, error) {
	envOnce.Do(func() {
		path := CredentialsPath()
		if path == "" {
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			envErr = fmt.Errorf("push: no es poden llegir les credencials FCM: %w", err)
			return
		}
		envSender, envErr = NewFCM(raw)
	})
	return envSender, envErr
}

// Status diu si el push és configurat i, si ho és però no es pot fer
// servir (fitxer il·legible, clau dolenta), l'error. Per a «gregal doctor».
func Status() (configured bool, err error) {
	if CredentialsPath() == "" {
		return false, nil
	}
	s, err := FromEnv()
	return s != nil, err
}

// DefaultFile és el nom del fitxer de credencials que es busca a la carpeta
// de dades quan no hi ha GREGAL_FCM_CREDENTIALS: així l'app d'escriptori, que
// no té variables d'entorn pròpies, l'agafa només deixant-hi el fitxer.
const DefaultFile = "fcm-service-account.json"

// CredentialsPath és el fitxer de credencials a fer servir: la variable
// d'entorn si hi és; si no, <carpeta de dades>/fcm-service-account.json si
// existeix; si no, "" (sense push).
func CredentialsPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvCredentials)); p != "" {
		return p
	}
	p := filepath.Join(profile.DataDir(), DefaultFile)
	if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
		return p
	}
	return ""
}

// NewFCM crea el Sender a partir del JSON del compte de servei.
func NewFCM(credentials []byte) (*FCM, error) {
	var acct serviceAccount
	if err := json.Unmarshal(credentials, &acct); err != nil {
		return nil, errors.New("push: credencials FCM il·legibles")
	}
	if acct.ProjectID == "" || acct.ClientEmail == "" || acct.PrivateKey == "" {
		return nil, errors.New("push: a les credencials FCM hi falta project_id, client_email o private_key")
	}
	if acct.TokenURI == "" {
		acct.TokenURI = "https://oauth2.googleapis.com/token"
	}
	block, _ := pem.Decode([]byte(acct.PrivateKey))
	if block == nil {
		return nil, errors.New("push: clau privada FCM il·legible")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		if k, err2 := x509.ParsePKCS1PrivateKey(block.Bytes); err2 == nil {
			parsed = k
		} else {
			return nil, errors.New("push: clau privada FCM il·legible")
		}
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("push: la clau FCM ha de ser RSA")
	}
	return &FCM{
		acct:     acct,
		key:      key,
		client:   &http.Client{Timeout: 15 * time.Second},
		endpoint: "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(acct.ProjectID) + "/messages:send",
	}, nil
}

// accessToken bescanvia un JWT signat pel token OAuth (cache fins que caduca).
func (f *FCM) accessToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && time.Now().Before(f.expires) {
		return f.token, nil
	}
	now := time.Now()
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{
		"iss":   f.acct.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   f.acct.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	unsigned := header + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(nil, f.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {unsigned + "." + enc.EncodeToString(sig)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.acct.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.AccessToken == "" {
		// Cap cos de resposta al missatge: pot portar detalls del compte.
		return "", fmt.Errorf("push: OAuth FCM ha fallat (%d)", resp.StatusCode)
	}
	f.token = out.AccessToken
	f.expires = now.Add(time.Duration(out.ExpiresIn)*time.Second - time.Minute)
	return f.token, nil
}

// Send envia un missatge de dades d'alta prioritat (desperta l'app en Doze
// dins dels límits de FCM). Només data, sense bloc «notification»: l'app
// decideix què mostrar després de llegir el contingut autenticat.
func (f *FCM) Send(ctx context.Context, token string, data map[string]string) error {
	access, err := f.accessToken(ctx)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"message": map[string]any{
		"token":   token,
		"data":    data,
		"android": map[string]any{"priority": "high", "ttl": "86400s"},
	}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusNotFound || bytes.Contains(raw, []byte("UNREGISTERED")):
		return ErrUnregistered
	default:
		return fmt.Errorf("push: FCM ha respost %d", resp.StatusCode)
	}
}
