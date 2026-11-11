package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/push"
)

// Push de fons per a l'app (B7). Quan un vent parla sol i el mòbil ha de
// sonar, el servidor envia als dispositius del compte un push de FCM que
// només porta l'id opac de la novetat; l'app la llegeix amb la seva sessió
// (GET /api/winds/novetats?id=…). Sense credencials FCM (GREGAL_FCM_CREDENTIALS)
// no s'envia res: el mode és «polling» i l'app ho sap per GET /api/v2/push.
//
// Els tokens dels dispositius viuen a push-devices.json del dir del compte
// (0600) i no surten mai per cap resposta ni cap log.

const (
	pushDevicesFile = "push-devices.json"
	maxPushDevices  = 8
)

type pushDevice struct {
	Token    string `json:"token"`
	Platform string `json:"platform,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Since    int64  `json:"since"`
}

// pushSender torna el Sender configurat (nil = sense push). Els tests el canvien.
var pushSender = func() push.Sender {
	s, err := push.FromEnv()
	if err != nil {
		pushConfigWarn.Do(func() { log.Printf("push desactivat: %v", err) })
		return nil
	}
	return s
}

var pushConfigWarn sync.Once

// pushDevicesMu protegeix el fitxer de dispositius també entre processos.
var pushDevicesMu = &crossLock{name: ".push-devices.lock"}

func pushDevicesPath(s *Server) string { return filepath.Join(s.sessDir(), pushDevicesFile) }

func loadPushDevices(s *Server) []pushDevice {
	raw, err := os.ReadFile(pushDevicesPath(s))
	if err != nil {
		return nil
	}
	var list []pushDevice
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	return list
}

func savePushDevices(s *Server, list []pushDevice) {
	raw, err := json.Marshal(list)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(pushDevicesPath(s)), 0o700)
	_ = writeFileAtomic(pushDevicesPath(s), raw)
	_ = os.Chmod(pushDevicesPath(s), 0o600)
}

// myPushDevices són els dispositius del compte que pregunta.
func myPushDevices(s *Server) []pushDevice {
	me := s.usuari()
	pushDevicesMu.Lock()
	all := loadPushDevices(s)
	pushDevicesMu.Unlock()
	out := all[:0:0]
	for _, d := range all {
		if d.Owner == me && d.Token != "" {
			out = append(out, d)
		}
	}
	return out
}

func forgetPushToken(s *Server, token string) {
	pushDevicesMu.Lock()
	defer pushDevicesMu.Unlock()
	list := loadPushDevices(s)
	out := list[:0]
	for _, d := range list {
		if d.Token != token {
			out = append(out, d)
		}
	}
	savePushDevices(s, out)
}

// pushNovetat avisa els dispositius del compte d'una novetat nova. Mai no
// bloqueja el torn: s'envia en segon pla i els errors només es registren
// sense token ni contingut.
func (s *Server) pushNovetat(n windNovetat) {
	sender := pushSender()
	if sender == nil || n.ID == "" {
		return
	}
	devices := myPushDevices(s)
	if len(devices) == 0 {
		return
	}
	data := map[string]string{"k": eventNovetat, "id": n.ID}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, d := range devices {
			err := sender.Send(ctx, d.Token, data)
			if errors.Is(err, push.ErrUnregistered) {
				forgetPushToken(s, d.Token)
			} else if err != nil {
				log.Printf("push: no s'ha pogut avisar un dispositiu: %v", err)
			}
		}
	}()
}

// handlePush: GET diu si hi ha push i quants dispositius té el compte; POST
// registra el token del dispositiu; DELETE l'oblida (sortir del compte).
func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		enabled := pushSender() != nil
		mode := "polling"
		if enabled {
			mode = "fcm"
		}
		writeJSON(w, map[string]any{"enabled": enabled, "mode": mode, "devices": len(myPushDevices(s))})
	case http.MethodPost, http.MethodDelete:
		var req struct {
			Token    string `json:"token"`
			Platform string `json:"platform"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			http.Error(w, "json invàlid", http.StatusBadRequest)
			return
		}
		token := strings.TrimSpace(req.Token)
		if token == "" || len(token) > 4096 || strings.ContainsAny(token, " \r\n\t") {
			http.Error(w, "token invàlid", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodDelete {
			forgetPushToken(s, token)
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		me := s.usuari()
		pushDevicesMu.Lock()
		list := loadPushDevices(s)
		out := make([]pushDevice, 0, len(list)+1)
		for _, d := range list {
			// El mateix token d'abans (fins i tot d'un altre compte en
			// aquest dispositiu) se substitueix: un mòbil, un propietari.
			if d.Token != token {
				out = append(out, d)
			}
		}
		out = append(out, pushDevice{Token: token, Platform: strings.TrimSpace(req.Platform), Owner: me, Since: time.Now().Unix()})
		if len(out) > maxPushDevices {
			out = out[len(out)-maxPushDevices:]
		}
		savePushDevices(s, out)
		pushDevicesMu.Unlock()
		writeJSON(w, map[string]any{"ok": true, "enabled": pushSender() != nil})
	default:
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
	}
}
