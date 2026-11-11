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
		Dir        string `json:"dir"`
		Title      string `json:"title"`
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
		Preview    bool   `json:"preview"`
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
	if name := strings.ToLower(strings.TrimSpace(a.Name)); name != "" {
		return s.runPublishNamed(ctx, dir, name, strings.ToLower(strings.TrimSpace(a.Visibility)), a.Title, a.Preview)
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

// runPublishNamed publica una carpeta com a web amb nom (webs.go). El
// primer «public» d'un nom nou només passa si la persona l'ha confirmat
// (decisió del fil, plan.go): sense confirmació queda privada.
func (s *Server) runPublishNamed(ctx context.Context, dir, name, vis, title string, preview bool) (string, error) {
	if err := validWebName(name); err != nil {
		return "", err
	}
	if !s.canPublish() {
		return "", errWebCannot
	}
	if w := getWeb(name); w != nil && w.Owner != s.remindOwner() {
		return "", fmt.Errorf("the name %q is taken by another account: choose another name", name)
	}
	files, err := s.siteFilesFromDir(dir)
	if err != nil {
		return "", err
	}
	src, _ := s.resolveToolPath(dir)
	publicOK := false
	if vis == webPublic {
		w := getWeb(name)
		publicOK = (w != nil && w.PublicConfirmed) || consumeWebPublic(s.remindOwner(), name)
	}
	res, err := s.webPublish(webPublishOpts{Name: name, Visibility: vis, Title: strings.TrimSpace(title), Source: src,
		Stage: preview, Base: linkBaseOf(ctx), PublicOK: publicOK}, files)
	if err != nil {
		return "", err
	}
	out := s.webReceipt(res)
	urls := s.webURLs(res.Site, linkBaseOf(ctx))
	if preview {
		return out + "\nPrivate preview (owner only; open it from the Webs list): " + urls["previewUrl"] +
			"\nTo publish it, call publish again without preview.", nil
	}
	if res.Check != nil && !res.Check.OK {
		return out + "\nFix the problems and publish again; nothing changed for visitors.", nil
	}
	out += "\nLink: " + urls["url"]
	if res.Held {
		out += "\nIt was published PRIVATE: making a name public the first time needs the user's confirmation. Ask them; they can also switch it to public from the Webs list."
	}
	return out, nil
}
