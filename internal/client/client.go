// Package client és el client Go mínim del servei Gregal.
//
// Comença per la descoberta i la negociació de protocol. Les operacions de
// conversa i els events s'hi afegiran sobre la mateixa identitat validada.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gregal/internal/service"
)

const Protocol = service.Protocol

type Client struct {
	BaseURL    string
	Token      string
	InstanceID string
	HTTP       *http.Client
	SessionID  string
}

type Health struct {
	Status       string          `json:"status"`
	Protocol     string          `json:"protocol"`
	InstanceID   string          `json:"instance_id"`
	Capabilities map[string]bool `json:"capabilities"`
}

type Event struct {
	ID        uint64          `json:"id"`
	At        time.Time       `json:"at"`
	SessionID string          `json:"session_id,omitempty"`
	RunID     int             `json:"run_id,omitempty"`
	Kind      string          `json:"kind"`
	Text      string          `json:"text"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type EventsPage struct {
	Events []Event `json:"events"`
	Cursor uint64  `json:"cursor"`
	Next   uint64  `json:"next"`
}

// ErrEventsStreamUnsupported indica que el servidor és anterior a la ruta SSE
// durable. El TUI pot canviar a Events (polling) sense perdre el cursor.
var ErrEventsStreamUnsupported = errors.New("stream d'events no disponible")

type Session struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Busy  bool   `json:"busy"`
	Mode  string `json:"mode"`
	Role  string `json:"role"`
	CWD   string `json:"cwd"`
}

// Run és l'estat d'una execució de la cua compartida. State segueix el
// cicle queued → running → completed | failed | cancelled.
type Run struct {
	ID         int64     `json:"id"`
	State      string    `json:"state"`
	Session    string    `json:"session,omitempty"`
	Workspace  string    `json:"workspace,omitempty"`
	Error      string    `json:"error,omitempty"`
	EnqueuedAt time.Time `json:"enqueued_at,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

type runPage struct {
	Run    Run    `json:"run"`
	Cursor uint64 `json:"cursor"`
}

// Submission preserves the starting cursor needed to follow only this turn.
type Submission = runPage

// ForSession returns a copy scoped to one conversation, including run lookups
// and cancellation. The original client remains safe to use for other sessions.
func (c *Client) ForSession(id string) *Client {
	scoped := *c
	scoped.SessionID = id
	return &scoped
}

type sessionsPage struct {
	Sessions []Session `json:"sessions"`
}

func New(baseURL, token, instanceID string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), Token: token,
		InstanceID: instanceID, HTTP: &http.Client{Timeout: 5 * time.Second},
	}
}

func Discover(ctx context.Context) (*Client, Health, error) {
	d, err := service.Read()
	if err != nil {
		return nil, Health{}, fmt.Errorf("servei no descobert: %w", err)
	}
	c := New(d.Address, d.Token, d.Instance)
	h, err := c.Health(ctx)
	if err != nil {
		return nil, Health{}, err
	}
	if h.Protocol != Protocol {
		return nil, h, fmt.Errorf("protocol incompatible: %s", h.Protocol)
	}
	if d.Instance != "" && h.InstanceID != d.Instance {
		return nil, h, fmt.Errorf("la instància ha canviat: descriptor %s, servei %s", d.Instance, h.InstanceID)
	}
	return c, h, nil
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	var out Health
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("client de servei buit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/health", nil)
	if err != nil {
		return out, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("resposta de salut il·legible: %w", err)
	}
	if out.Status != "ok" || out.InstanceID == "" {
		return out, fmt.Errorf("servei no saludable")
	}
	return out, nil
}

// Events llegeix els events persistents posteriors a after. El cursor next
// es pot desar i passar a la següent crida després d'una reconnexió.
func (c *Client) Events(ctx context.Context, after uint64, limit int, sessionID string) (EventsPage, error) {
	if sessionID == "" && c != nil {
		sessionID = c.SessionID
	}
	var out EventsPage
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("client de servei buit")
	}
	q := url.Values{}
	q.Set("after", strconv.FormatUint(after, 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if sessionID != "" {
		q.Set("session", sessionID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/v2/events?"+q.Encode(), nil)
	if err != nil {
		return out, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("events il·legibles: %w", err)
	}
	return out, nil
}

// EventsStream llegeix el primer lot d'events d'un stream SSE durable. Torna
// tan aviat com rep un event perquè el consumidor pugui pintar-lo i reconnectar
// amb el cursor següent; si la ruta no existeix retorna ErrEventsStreamUnsupported.
func (c *Client) EventsStream(ctx context.Context, after uint64, limit int, sessionID string) (EventsPage, error) {
	if sessionID == "" && c != nil {
		sessionID = c.SessionID
	}
	var out EventsPage
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("client de servei buit")
	}
	q := url.Values{}
	q.Set("after", strconv.FormatUint(after, 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if sessionID != "" {
		q.Set("session", sessionID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/v2/events/stream?"+q.Encode(), nil)
	if err != nil {
		return out, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if after > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatUint(after, 10))
	}
	// New() usa un timeout curt per JSON; un stream viu ha de quedar obert i
	// queda limitat només pel context del consumidor.
	hc := *c.http()
	hc.Timeout = 0
	res, err := hc.Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusNotImplemented {
		res.Body.Close()
		return out, ErrEventsStreamUnsupported
	}
	if res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	if ct := strings.ToLower(res.Header.Get("Content-Type")); !strings.Contains(ct, "text/event-stream") {
		return out, ErrEventsStreamUnsupported
	}

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 4096), 2*1024*1024)
	var eventName, eventID string
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			eventName, eventID = "", ""
			return nil
		}
		var event Event
		if err := json.Unmarshal([]byte(data.String()), &event); err != nil {
			return fmt.Errorf("event SSE il·legible: %w", err)
		}
		if event.ID == 0 && eventID != "" {
			event.ID, _ = strconv.ParseUint(eventID, 10, 64)
		}
		if event.Kind == "" {
			event.Kind = eventName
		}
		if event.ID == 0 {
			return fmt.Errorf("event SSE sense cursor")
		}
		out.Events = append(out.Events, event)
		out.Next = event.ID
		eventName, eventID = "", ""
		data.Reset()
		return nil
	}
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return out, err
			}
			if len(out.Events) > 0 {
				return out, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // heartbeat
		}
		switch {
		case strings.HasPrefix(line, "id:"):
			eventID = strings.TrimSpace(line[3:])
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(line[5:]))
		}
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if err := flush(); err != nil {
		return out, err
	}
	if len(out.Events) > 0 {
		return out, nil
	}
	return out, fmt.Errorf("stream d'events tancat sense events")
}

func (c *Client) Sessions(ctx context.Context) ([]Session, error) {
	var out sessionsPage
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("client de servei buit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/sessions/live", nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}

func (c *Client) Cancel(ctx context.Context, sessionID string) error {
	if sessionID == "" && c != nil {
		sessionID = c.SessionID
	}
	if c == nil || c.BaseURL == "" {
		return fmt.Errorf("client de servei buit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/agent/cancel", strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if sessionID != "" {
		req.Header.Set("X-Gregal-Session", sessionID)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	return nil
}

// SubmitRun encua un torn a la cua compartida del servei (API v2). La
// resposta retorna aviat, amb l'execució en estat queued: els events del
// torn arriben després per Events amb el cursor. sessionID buit = sessió
// default del servei; idempotencyKey buit = sense deduplicació; workspace
// buit = el de la sessió del servei.
func (c *Client) SubmitRun(ctx context.Context, sessionID, task, mode, idempotencyKey, workspace string) (Run, error) {
	submitted, err := c.Submit(ctx, sessionID, task, mode, idempotencyKey, workspace)
	return submitted.Run, err
}

// Submit retains the initial event cursor, unlike the legacy SubmitRun method.
func (c *Client) Submit(ctx context.Context, sessionID, task, mode, idempotencyKey, workspace string) (Submission, error) {
	var out Submission
	if sessionID == "" && c != nil {
		sessionID = c.SessionID
	}
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("client de servei buit")
	}
	if strings.TrimSpace(task) == "" {
		return out, fmt.Errorf("tasca buida")
	}
	body := map[string]string{"task": task}
	if mode != "" {
		body["mode"] = mode
	}
	if idempotencyKey != "" {
		body["idempotency_key"] = idempotencyKey
	}
	// El directori del client delegat: sense això el torn corre al cwd de
	// la sessió del servei, no on l'usuari ha obert el TUI.
	if strings.TrimSpace(workspace) != "" {
		body["workspace"] = workspace
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return out, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v2/runs", bytes.NewReader(raw))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if sessionID != "" {
		req.Header.Set("X-Gregal-Session", sessionID)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusAccepted && res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	var page runPage
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return out, fmt.Errorf("execució il·legible: %w", err)
	}
	return page, nil
}

// Run consulta l'estat d'una execució de la cua.
func (c *Client) Run(ctx context.Context, id int64) (Run, error) {
	var out Run
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("client de servei buit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/v2/runs/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return out, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.SessionID != "" {
		req.Header.Set("X-Gregal-Session", c.SessionID)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	var page runPage
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return out, fmt.Errorf("execució il·legible: %w", err)
	}
	return page.Run, nil
}

// CancelRun cancel·la una execució identificada de la cua.
func (c *Client) CancelRun(ctx context.Context, id int64) error {
	if c == nil || c.BaseURL == "" {
		return fmt.Errorf("client de servei buit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v2/runs/"+strconv.FormatInt(id, 10)+"/cancel", strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.SessionID != "" {
		req.Header.Set("X-Gregal-Session", c.SessionID)
	}
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("servei: HTTP %d", res.StatusCode)
	}
	return nil
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
