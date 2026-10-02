package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gregal/internal/tools"
)

// maxDocumentBytes és el que un bot pot baixar amb getFile. Per sobre, ni ho
// intentem: val més dir-ho clar que fallar a mitges.
const maxDocumentBytes = 20 << 20

// handleDocument desa un fitxer enviat al xat dins la carpeta de treball de
// la sessió i passa la RUTA a l'agent, que ja sap llegir-lo amb les eines que
// té (pdftotext per a PDF, python3 per a Office, unzip, read...).
//
// Per què una ruta i no el contingut: un PDF de 10 MB no cap al context, i
// l'agent el pot consultar per trossos. A més, el fitxer queda al disc de la
// persona, dins el seu workspace, i el guard de rutes el deixa llegir.
//
// Les imatges enviades COM A document ("envia sense comprimir") van pel camí
// de les fotos: el model les veu directament.
func (b *Bot) handleDocument(ctx context.Context, m *Message, st *chatState, text string) {
	if m.Chat.Type != "private" && !b.addressed(m) {
		return
	}
	doc := m.Document
	if doc == nil {
		doc = m.Audio
	}
	if doc == nil {
		return
	}
	caption := strings.TrimSpace(m.Caption)
	if caption == "" {
		caption = strings.TrimSpace(text)
	}
	if caption == "" {
		caption = b.tr("Llegeix aquest document i fes-me'n un resum.", "Read this document and summarize it for me.")
	}
	b.logf("document: usuari %d xat %d (%s) %q tipus %s %d bytes",
		m.From.ID, m.Chat.ID, m.Chat.Type, truncate(doc.FileName, 50), doc.MimeType, doc.FileSize)
	if doc.FileSize > maxDocumentBytes {
		b.send(ctx, m.Chat.ID, fmt.Sprintf(
			b.tr("⚠️ Aquest fitxer fa %s i el màxim que puc baixar són 20 MB.\nPuja'l a Nextcloud i digue'm la ruta.", "⚠️ This file is %s; the download limit is 20 MB.\nUpload it to Nextcloud and send me the path."),
			humanBytes(doc.FileSize)))
		return
	}
	fp, err := b.api.GetFile(ctx, doc.FileID)
	if err != nil {
		b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("No he pogut obtenir el fitxer: ", "Could not get the file: ")+err.Error())
		return
	}
	raw, err := b.api.Download(ctx, fp)
	if err != nil {
		b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("No he pogut descarregar el fitxer: ", "Could not download the file: ")+err.Error())
		return
	}
	// Imatge enviada com a document: va al camí de les fotos.
	if strings.HasPrefix(doc.MimeType, "image/") {
		if u, err := tools.DataURLForBytes(raw, "imatge Telegram"); err == nil {
			b.turn(ctx, m.Chat.ID, st, caption, u)
			return
		}
	}
	abs, err := b.saveDocument(st, m.Chat.ID, doc, raw)
	if err != nil {
		b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("No he pogut desar el fitxer: ", "Could not save the file: ")+err.Error())
		return
	}
	b.send(ctx, m.Chat.ID, fmt.Sprintf("📎 "+b.tr("%s (%s) desat a %s", "%s (%s) saved to %s"),
		filepath.Base(abs), humanBytes(len(raw)), b.relative(st, abs)))
	// El torn porta la caption de la persona I la ruta, amb la instrucció de
	// llegir-lo: si només hi hagués la ruta, el model sovint respondria de
	// memòria sobre el nom del fitxer.
	b.turn(ctx, m.Chat.ID, st,
		caption+"\n\n[document desat a "+abs+" — llegeix-lo amb les eines abans de respondre]")
}

// documentDir és la carpeta on van els fitxers d'aquest xat: dins el
// workspace de qui parla, en una carpeta amagada, per no embrutar el
// projecte. Amb compte lligat, la carpeta és de l'usuari (la veu des del
// TUI/web a casa seva); sense, la d'abans (per xat).
func (b *Bot) documentDir(st *chatState, chatID int64) string {
	qui := strconv.FormatInt(chatID, 10)
	if st != nil && st.user != "" {
		qui = "usuari-" + st.user
	}
	return filepath.Join(b.dirOf(st), ".gregal", "documents", qui)
}

// saveDocument desa els bytes amb un nom segur i torna la ruta absoluta.
func (b *Bot) saveDocument(st *chatState, chatID int64, doc *Document, raw []byte) (string, error) {
	dir := b.documentDir(st, chatID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	abs := filepath.Join(dir, safeFileName(doc.FileName, doc.FileID, doc.MimeType))
	if err := os.WriteFile(abs, raw, 0o600); err != nil {
		return "", err
	}
	return abs, nil
}

// relative retalla la ruta perquè el missatge no sigui una filera de carpetes.
func (b *Bot) relative(st *chatState, abs string) string {
	if rel, err := filepath.Rel(b.dirOf(st), abs); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return abs
}

// safeFileName treu tot el que pot fer mal d'un nom que ve de fora: el nom
// sencer el tria el client, i amb un nom com «../../.ssh/authorized_keys»
// s'escriuria fora de la carpeta. Sempre filepath.Base, sense separadors, i
// si no en queda res, el file_id de Telegram.
func safeFileName(name, fileID, mime string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	base := filepath.Base(name)
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_' || r == ' ':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), " .")
	if out == "" || out == "." || out == ".." {
		out = "document-" + truncate(fileID, 12)
	}
	if !strings.Contains(out, ".") {
		if ext := extensionFor(mime); ext != "" {
			out += ext
		}
	}
	return out
}

// extensionFor dona una extensió quan el client no n'ha enviat cap, perquè
// les eines del sistema (pdftotext, unzip) treballen per extensió.
func extensionFor(mime string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0])) {
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "application/json":
		return ".json"
	case "application/zip":
		return ".zip"
	}
	return ""
}

// humanBytes fa llegible una mida.
func humanBytes(n int) string {
	if n <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}
