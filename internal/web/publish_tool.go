package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/agent"
)

// Eina `publish`: el vent (o l'agent) publica la carpeta d'una web o d'un
// joc estàtic amb el mateix sistema que el botó de publicar (share_site.go)
// i torna l'enllaç /share/<id>/. Tornar a publicar la mateixa carpeta
// actualitza el mateix enllaç (shareMeta.Source), així la persona no rep
// un enllaç nou a cada canvi.

func (s *Server) runPublishTool(ctx context.Context, workspace, argsJSON string) (string, error) {
	var a struct {
		Dir   string `json:"dir"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(agent.ResolveArgs(workspace, argsJSON)), &a); err != nil {
		return "", errors.New("unreadable arguments")
	}
	dir := strings.TrimSpace(a.Dir)
	if dir == "" {
		dir = workspace
	}
	if dir == "" {
		return "", errors.New("dir is required: the folder with index.html")
	}
	m, err := s.publishDir(dir, a.Title)
	if err != nil {
		return "", err
	}
	link := absLink(linkBaseOf(ctx), "/share/"+m.ID+"/")
	return fmt.Sprintf("Published %q (%d files, %d KB).\nPublic link (opens on any device, also the phone): %s\nFolder: %s\nPublishing this folder again updates the same link.", m.Title, m.Files, (m.Bytes+1023)/1024, link, dir), nil
}

// publishDir publica (o republica) una carpeta com a web.
func (s *Server) publishDir(dir, title string) (shareMeta, error) {
	files, err := s.siteFilesFromDir(dir)
	if err != nil {
		return shareMeta{}, err
	}
	abs, err := s.resolveToolPath(dir)
	if err != nil {
		return shareMeta{}, err
	}
	abs = filepath.Clean(abs)
	owner := s.usuari()
	now := time.Now().UTC()
	m := shareMeta{Kind: "site", Owner: owner, Created: now}
	n := 0
	for _, x := range listShares(owner) {
		if x.Kind != "site" {
			continue
		}
		n++
		if x.Source != "" && strings.EqualFold(filepath.Clean(x.Source), abs) && !siteExpired(x) {
			m = x
		}
	}
	if m.ID == "" {
		if n >= siteMaxPerOwner {
			return shareMeta{}, fmt.Errorf("at most %d published sites: unpublish one first", siteMaxPerOwner)
		}
		if m.ID, err = newShareID(); err != nil {
			return shareMeta{}, err
		}
	}
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if title != "" {
		m.Title = title
	}
	if m.Title == "" {
		m.Title = filepath.Base(abs)
	}
	m.Source = abs
	m.Updated = &now
	m.Files, m.Bytes = len(files), 0
	for _, f := range files {
		m.Bytes += len(f.data)
	}
	if err := writeSite(m, files); err != nil {
		return shareMeta{}, err
	}
	return m, nil
}
