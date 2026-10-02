package telegram

import (
	"context"
	"strings"

	"gregal/internal/tools"
)

// handlePhoto descarrega la mida més gran d'una foto i l'envia al torn
// com a imatge (caption o text per defecte). Errors visibles, mai silenci.
func (b *Bot) handlePhoto(ctx context.Context, m *Message, st *chatState, text string) {
	if m.Chat.Type != "private" && !b.addressed(m) {
		return
	}
	caption := strings.TrimSpace(m.Caption)
	if caption == "" {
		caption = text
	}
	if caption == "" {
		caption = b.tr("Descriu aquesta imatge en català.", "Describe this image.")
	}
	big := m.Photo[len(m.Photo)-1]
	b.logf("foto: usuari %d xat %d (%s) file %dx%d %q", m.From.ID, m.Chat.ID, m.Chat.Type, big.Width, big.Height, truncate(caption, 50))
	fp, err := b.api.GetFile(ctx, big.FileID)
	if err != nil {
		b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("No he pogut obtenir la foto: ", "Could not get the photo: ")+err.Error())
		return
	}
	raw, err := b.api.Download(ctx, fp)
	if err != nil {
		b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("No he pogut descarregar la foto: ", "Could not download the photo: ")+err.Error())
		return
	}
	u, err := tools.DataURLForBytes(raw, "foto Telegram")
	if err != nil {
		b.send(ctx, m.Chat.ID, "⚠️ "+b.tr("La foto no és vàlida: ", "Invalid photo: ")+err.Error())
		return
	}
	b.turn(ctx, m.Chat.ID, st, caption, u)
}
