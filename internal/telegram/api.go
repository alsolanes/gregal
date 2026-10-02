// Package telegram implementa un bot de Telegram natiu de Gregal: seguiment de
// sessions des del mòbil, obertura de sessions noves i aprovacions amb botons.
//
// La capa d'aquest fitxer és només la Bot API (HTTP + JSON), sense dependències.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL és l'endpoint de la Bot API.
const DefaultBaseURL = "https://api.telegram.org"

// API és un client mínim de la Bot API.
type API struct {
	Token    string
	BaseURL  string
	HTTP     *http.Client
	language string
}

// NewAPI crea el client amb el token donat.
func NewAPI(token string) *API {
	return &API{Token: token, BaseURL: DefaultBaseURL, language: "en", HTTP: &http.Client{
		Timeout: 90 * time.Second,
		// Els 429 i els 5xx de Telegram es reintenten sols (vegeu retry.go):
		// el streaming fa moltes edicions seguides i el limit de ritme es
		// normal, no ha d arribar mai a l usuari com un error.
		Transport: retryTransport{},
	}}
}

func (a *API) tr(ca, en string) string {
	if a != nil && a.language == "ca" {
		return ca
	}
	return en
}

// User és un usuari de Telegram.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

// Chat és una conversa (privada, grup o supergrup).
type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"` // private|group|supergroup|channel
	Title string `json:"title"`
}

// Message és un missatge.
type Message struct {
	MessageID int         `json:"message_id"`
	Chat      Chat        `json:"chat"`
	From      *User       `json:"from"`
	Text      string      `json:"text"`
	Caption   string      `json:"caption"`
	Photo     []PhotoSize `json:"photo,omitempty"`
	Voice     *Voice      `json:"voice,omitempty"`
	Document  *Document   `json:"document,omitempty"`
	Audio     *Document   `json:"audio,omitempty"`
	ReplyTo   *Message    `json:"reply_to_message"`
}

// PhotoSize és una mida d'una foto (Telegram n'envia diverses, de menor
// a major: l'última és la més gran).
type PhotoSize struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int    `json:"file_size"`
}

// Document és un fitxer enviat al xat: PDF, Office, text, imatge com a
// fitxer... Telegram n'envia el nom i el tipus, que és el que fa falta per
// desar-lo amb el nom correcte. Els fitxers d'àudio porten els mateixos camps.
type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int    `json:"file_size"`
}

// Voice és una nota de veu (1.0: detectada però no transcrita, sense STT local).
type Voice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
}

// CallbackQuery és la pulsació d'un botó en línia.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

// Update és una actualització del bot.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	EditedMessage *Message       `json:"edited_message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// InlineButton és un botó en línia.
type InlineButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

// Keyboard és una fila de botons.
type Keyboard [][]InlineButton

type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

// call fa la petició i deserialitza el resultat.
func (a *API) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := strings.TrimRight(a.BaseURL, "/") + "/bot" + a.Token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %s", method, redact(err.Error(), a.Token))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%s: %s", method, err)
	}
	var r apiResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("%s: %s (http %d)", method, a.tr("resposta il·legible", "unreadable response"), resp.StatusCode)
	}
	if !r.OK {
		return fmt.Errorf("%s: %s", method, redact(r.Description, a.Token))
	}
	if out != nil && len(r.Result) > 0 {
		if err := json.Unmarshal(r.Result, out); err != nil {
			return fmt.Errorf("%s: %s", method, a.tr("resultat il·legible", "unreadable result"))
		}
	}
	return nil
}

// redact treu el token de qualsevol missatge d'error.
func redact(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "<token>")
}

// GetFile resol el path de descàrrega d'un file_id (fotos, veu, documents).
func (a *API) GetFile(ctx context.Context, fileID string) (string, error) {
	var f struct {
		FilePath string `json:"file_path"`
	}
	if err := a.call(ctx, "getFile", map[string]any{"file_id": fileID}, &f); err != nil {
		return "", err
	}
	if f.FilePath == "" {
		return "", fmt.Errorf("getFile: %s", a.tr("sense file_path", "missing file_path"))
	}
	return f.FilePath, nil
}

// MaxDownloadBytes limita les descàrregues (mateix topall que read_image).
const MaxDownloadBytes = 8 << 20

// Download baixa un fitxer ja resolt per GetFile. Respeta BaseURL (testeable).
func (a *API) Download(ctx context.Context, filePath string) ([]byte, error) {
	url := strings.TrimRight(a.BaseURL, "/") + "/file/bot" + a.Token + "/" + strings.TrimLeft(filePath, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %s", redact(err.Error(), a.Token))
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download: http %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download: %s", err)
	}
	if len(raw) > MaxDownloadBytes {
		return nil, fmt.Errorf("download: "+a.tr("massa gran (màxim %d bytes)", "too large (%d bytes maximum)"), MaxDownloadBytes)
	}
	return raw, nil
}

// Me retorna el bot (comprovació de token).
func (a *API) Me(ctx context.Context) (User, error) {
	var u User
	err := a.call(ctx, "getMe", map[string]any{}, &u)
	return u, err
}

// GetUpdates fa long polling (timeout en segons).
func (a *API) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	p := map[string]any{
		"timeout":         timeout,
		"offset":          offset,
		"allowed_updates": []string{"message", "edited_message", "callback_query"},
	}
	var ups []Update
	if err := a.call(ctx, "getUpdates", p, &ups); err != nil {
		return nil, err
	}
	return ups, nil
}

// SendMessage envia text (amb teclat opcional) i retorna el missatge.
func (a *API) SendMessage(ctx context.Context, chatID int64, text string, kb Keyboard) (Message, error) {
	p := map[string]any{"chat_id": chatID, "text": text,
		"parse_mode":           "HTML",
		"link_preview_options": map[string]any{"is_disabled": true}}
	if len(kb) > 0 {
		p["reply_markup"] = map[string]any{"inline_keyboard": kb}
	}
	var m Message
	err := a.call(ctx, "sendMessage", p, &m)
	return m, err
}

// EditMessageText edita un missatge ja enviat (teclat opcional).
func (a *API) EditMessageText(ctx context.Context, chatID int64, msgID int, text string, kb Keyboard) error {
	p := map[string]any{"chat_id": chatID, "message_id": msgID, "text": text,
		"parse_mode":           "HTML",
		"link_preview_options": map[string]any{"is_disabled": true}}
	if len(kb) > 0 {
		p["reply_markup"] = map[string]any{"inline_keyboard": kb}
	}
	return a.call(ctx, "editMessageText", p, nil)
}

// BotCommand és una ordre per al menú del bot (setMyCommands).
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// SetMyCommands registra el menú d'ordres del bot (best effort: si falla,
// el bot continua funcionant amb /help).
func (a *API) SetMyCommands(ctx context.Context, cmds []BotCommand) error {
	return a.call(ctx, "setMyCommands", map[string]any{"commands": cmds}, nil)
}

// SendDocument envia un fitxer (multipart/form-data) amb subtítol opcional.
func (a *API) SendDocument(ctx context.Context, chatID int64, filename string, content []byte, caption string) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("chat_id", itoa64(chatID))
	if caption != "" {
		_ = w.WriteField("caption", caption)
		_ = w.WriteField("parse_mode", "HTML")
	}
	part, err := w.CreateFormFile("document", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	url := strings.TrimRight(a.BaseURL, "/") + "/bot" + a.Token + "/sendDocument"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %s", "sendDocument", redact(err.Error(), a.Token))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%s: %s", "sendDocument", err)
	}
	var r apiResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("sendDocument: %s (http %d)", a.tr("resposta il·legible", "unreadable response"), resp.StatusCode)
	}
	if !r.OK {
		return fmt.Errorf("sendDocument: %s", redact(r.Description, a.Token))
	}
	return nil
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// SendChatAction mostra "escrivint…" (action: typing).
func (a *API) SendChatAction(ctx context.Context, chatID int64, action string) error {
	return a.call(ctx, "sendChatAction", map[string]any{"chat_id": chatID, "action": action}, nil)
}

// AnswerCallbackQuery tanca el "rellotge" del botó.
func (a *API) AnswerCallbackQuery(ctx context.Context, id, text string) error {
	return a.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}
