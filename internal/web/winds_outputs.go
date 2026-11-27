package web

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gregal/internal/gdocs"
)

// El prestatge d'un vent: el que ha fet al seu xat (documents vius creats o
// canviats, webs publicades, fitxers escrits) perquè no es perdi entre els
// missatges. Cada torn 1:1 apunta a wind-outputs.json (per compte) quin vent ha
// fet què; títol i enllaç es llegeixen en mostrar-lo (l'esborrat ja no surt).
//
//	GET /api/winds/outputs?wind=<id> → {items:[{at,kind,title,url}]}

const windOutputsMax = 500

var (
	windOutputsMu sync.Mutex
	shareLinkRe   = regexp.MustCompile(`/share/([0-9a-f]{40})/`)
)

// windOutputRec és una línia del registre: kind és doc, share, web o file i ref
// l'id del document o de l'enllaç, o el nom de la web o del fitxer.
type windOutputRec struct {
	At   time.Time `json:"at"`
	Wind string    `json:"wind"`
	Kind string    `json:"kind"`
	Ref  string    `json:"ref"`
}

// WindOutput és una peça del prestatge.
type WindOutput struct {
	At    time.Time `json:"at"`
	Kind  string    `json:"kind"` // doc | deck | sheet | site | link | file
	Title string    `json:"title"`
	URL   string    `json:"url"`
}

func (s *Server) windOutputsPath() string {
	return filepath.Join(s.windStateDir(), "wind-outputs.json")
}

func (s *Server) loadWindOutputs() ([]windOutputRec, error) {
	raw, err := os.ReadFile(s.windOutputsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var recs []windOutputRec
	return recs, json.Unmarshal(raw, &recs)
}

// noteWindOutput apunta el que ha deixat una eina en un torn 1:1 d'un vent.
func (s *Server) noteWindOutput(ctx context.Context, name, argsJSON, out string) {
	k, ok := windSteerKeyOf(ctx)
	if !ok || k.kind != "wind" {
		return
	}
	var kind, ref string
	switch name {
	case "doc":
		kind, ref = "doc", docChangedID(argsJSON, out)
	case "publish":
		var a struct{ Name string }
		if m := shareLinkRe.FindStringSubmatch(out); m != nil {
			kind, ref = "share", m[1]
		} else if json.Unmarshal([]byte(argsJSON), &a) == nil {
			if n := strings.ToLower(strings.TrimSpace(a.Name)); n != "" && getWeb(n) != nil {
				kind, ref = "web", n
			}
		}
	case "pdf_create", "pdf_edit", "office_create", "office_edit":
		dir := windDocsOf(ctx)
		for _, t := range windDocTargets(dir, name, argsJSON) {
			if dir != "" && inside(dir, t) {
				s.recordWindOutput(k.id, "file", filepath.Base(t))
			}
		}
		return
	}
	if ref != "" {
		s.recordWindOutput(k.id, kind, ref)
	}
}

// recordWindOutput posa la peça a dalt de tot (si ja hi era, la mou).
func (s *Server) recordWindOutput(wind, kind, ref string) {
	windOutputsMu.Lock()
	defer windOutputsMu.Unlock()
	recs, err := s.loadWindOutputs()
	if err != nil {
		log.Printf("prestatge del vent: %v", err)
		return
	}
	out := []windOutputRec{{At: time.Now().UTC(), Wind: wind, Kind: kind, Ref: ref}}
	for _, r := range recs {
		if r.Wind == wind && r.Kind == kind && r.Ref == ref {
			continue
		}
		if len(out) == windOutputsMax {
			break
		}
		out = append(out, r)
	}
	if err := writeWindJSON(s.windOutputsPath(), out); err != nil {
		log.Printf("prestatge del vent: %v", err)
	}
}

var docExportFormat = map[string]string{gdocs.KindDoc: "docx", gdocs.KindDeck: "pptx", gdocs.KindSheet: "xlsx"}

// windOutput resol una línia del registre; false si ja no hi és o no és
// d'aquest compte.
func (s *Server) windOutput(r windOutputRec, docs *gdocs.Store) (WindOutput, bool) {
	o := WindOutput{At: r.At}
	switch r.Kind {
	case "doc":
		d, err := docs.Get(r.Ref)
		if err != nil {
			return o, false
		}
		o.Kind, o.Title = d.Kind, d.Title
		o.URL = "/api/docs/" + d.ID + "/export?scope=mine&format=" + docExportFormat[d.Kind]
	case "share":
		m, err := readShareMeta(r.Ref)
		if err != nil || m.Owner != s.usuari() || siteExpired(m) {
			return o, false
		}
		o.Kind, o.Title, o.URL = "link", m.Title, "/share/"+m.ID
		if m.Kind == "site" {
			o.Kind, o.URL = "site", o.URL+"/"
		}
	case "web":
		w := getWeb(r.Ref)
		if w == nil || w.Owner != s.remindOwner() {
			return o, false
		}
		urls := s.webURLs(w, "")
		o.Kind, o.Title, o.URL = "site", firstNonEmpty(w.Title, w.Name), firstNonEmpty(urls["url"], urls["path"])
	case "file":
		if fi, err := os.Stat(filepath.Join(s.windDocsDir(), r.Ref)); err != nil || fi.IsDir() {
			return o, false
		}
		o.Kind, o.Title, o.URL = "file", r.Ref, "/api/winds/doc?name="+url.QueryEscape(r.Ref)
	default:
		return o, false
	}
	return o, true
}

func (s *Server) handleWindOutputs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	wind := strings.TrimSpace(r.URL.Query().Get("wind"))
	if wind == "" {
		http.Error(w, "wind required", http.StatusBadRequest)
		return
	}
	windOutputsMu.Lock()
	recs, err := s.loadWindOutputs()
	windOutputsMu.Unlock()
	docs, derr := gdocs.OpenAt(s.windDocsDir())
	if err != nil || derr != nil {
		http.Error(w, "could not read the shelf", http.StatusInternalServerError)
		return
	}
	// El registre ja és del més nou al més vell (recordWindOutput).
	items := []WindOutput{}
	for _, rec := range recs {
		if rec.Wind != wind {
			continue
		}
		if o, ok := s.windOutput(rec, docs); ok {
			items = append(items, o)
		}
	}
	writeJSON(w, map[string]any{"items": items})
}
