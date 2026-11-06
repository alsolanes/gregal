package web

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
)

// Publicar el que ha fet una colla.
//
// Cada membre treballa en una còpia privada del projecte i no té l'eina
// publish (publicar la còpia seria publicar una versió que no és la final).
// En acabar, el coordinador diu quina carpeta del projecte s'ha de posar en
// línia i el servidor la publica un cop els fitxers ja hi són. Abans, una web
// d'una colla mai quedava publicada i la colla podia gastar voltes provant-ho.

type crewPublishReq struct {
	Dir   string `json:"dir"`
	Title string `json:"title"`
}

// crewPublish publica la carpeta (relativa a la carpeta de treball dels vents,
// mai a fora) i torna la línia amb l'enllaç per a la conclusió, o "" si no s'ha
// pogut (queda al registre del servidor).
func (s *Server) crewPublish(ctx context.Context, req crewPublishReq, ca bool) string {
	ws, _ := s.ventWorkspace()
	dir := strings.TrimSpace(req.Dir)
	if ws == "" || dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(ws, dir)
	}
	dir = filepath.Clean(dir)
	if rel, err := filepath.Rel(ws, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		log.Printf("crew publish: %q is outside the working folder", req.Dir)
		return ""
	}
	args, _ := json.Marshal(map[string]string{"dir": dir, "title": strings.TrimSpace(req.Title)})
	out, err := s.runBuildTool(ctx, ws, "publish", string(args))
	if err != nil {
		log.Printf("crew publish %s: %s", dir, redactError(err))
		return ""
	}
	link := ""
	for _, l := range strings.Split(out, "\n") {
		// «Public link (…): <url>»; sense adreça base coneguda l'enllaç és relatiu (/share/…).
		if strings.HasPrefix(l, "Public link") {
			if i := strings.LastIndex(l, ": "); i >= 0 {
				link = strings.TrimSpace(l[i+2:])
			}
			break
		}
	}
	if link == "" {
		return ""
	}
	if ca {
		return "Publicada: " + link
	}
	return "Published: " + link
}
