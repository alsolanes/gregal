package tools

// Pestanyes del navegador de l'agent. Amb el Chrome que engega el gregal
// (headless o finestra) es fan servir els endpoints HTTP del port de
// depuració (/json/list, /json/new, /json/activate, /json/close) i canviar de
// pestanya és obrir el websocket d'una altra. Amb el Chrome de l'usuari ho fa
// l'extensió (browser_ext.go), només dins el grup de pestanyes «Gregal». El
// panell de l'app d'escriptori té una sola pestanya.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type tabInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
	WS    string `json:"webSocketDebuggerUrl"`
}

var errNoTabs = fmt.Errorf("browser: the desktop app's browser pane has a single tab (use chrome:true for your Chrome, or browser.mode window/headless)")

func (b *Browser) extT() *extTransport {
	et, _ := b.t.(*extTransport)
	return et
}

func (b *Browser) tabList() ([]tabInfo, error) {
	if et := b.extT(); et != nil {
		return et.hub.tabs(b.ctx())
	}
	if b.port == 0 {
		return nil, errNoTabs
	}
	resp, err := cdpHTTP.Get(b.httpBase() + "/json/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var all, out []tabInfo
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, err
	}
	for _, t := range all {
		if t.Type == "page" {
			out = append(out, t)
		}
	}
	return out, nil
}

// tabIDs és la foto de pestanyes abans d'un clic (nil si no se'n sap).
func (b *Browser) tabIDs() map[string]bool {
	list, err := b.tabList()
	if err != nil {
		return nil
	}
	m := map[string]bool{}
	for _, t := range list {
		m[t.ID] = true
	}
	return m
}

// followNewTab: si el clic ha obert una pestanya (target=_blank, finestra
// emergent), l'agent hi passa i ho diu.
func (b *Browser) followNewTab(before map[string]bool) string {
	if before == nil {
		return ""
	}
	list, err := b.tabList()
	if err != nil {
		return ""
	}
	for _, t := range list {
		if !before[t.ID] {
			if err := b.selectTab(t); err != nil {
				return "The click opened a new tab (" + t.URL + ") but Gregal could not switch to it: " + err.Error()
			}
			_ = b.esperaCarrega()
			return "The click opened a new tab: now working in it (" + b.On() + "). Use tabs to list them."
		}
	}
	return ""
}

// Tabs llista les pestanyes de l'agent.
func (b *Browser) Tabs() (string, error) {
	list, err := b.tabList()
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for i, t := range list {
		cur := ""
		if t.ID == b.tabID {
			cur = "  ← current"
		}
		fmt.Fprintf(&out, "[%d] %s · %s%s\n", i+1, truncateInline(t.Title, 80), truncateInline(t.URL, 140), cur)
	}
	if out.Len() == 0 {
		return "No tabs.", nil
	}
	return out.String() + "Switch with tab_select tab:\"N\".", nil
}

func (b *Browser) findTab(sel string) (tabInfo, error) {
	list, err := b.tabList()
	if err != nil {
		return tabInfo{}, err
	}
	sel = strings.Trim(strings.TrimSpace(sel), "[]")
	if n, err := strconv.Atoi(sel); err == nil && n >= 1 && n <= len(list) {
		return list[n-1], nil
	}
	for _, t := range list {
		if t.ID == sel {
			return t, nil
		}
	}
	return tabInfo{}, fmt.Errorf("browser: no tab %q (tabs lists them)", sel)
}

func (b *Browser) selectTab(t tabInfo) error {
	b.mu.Lock()
	b.ax = nil
	b.mu.Unlock()
	if et := b.extT(); et != nil {
		id, _ := strconv.Atoi(t.ID)
		nt, err := et.hub.selectTab(b.ctx(), id)
		if err != nil {
			return err
		}
		b.t = nt
		b.tabID = t.ID
		return b.prepara()
	}
	if t.WS == "" {
		return fmt.Errorf("browser: the tab is busy with another debugger")
	}
	if b.mode == BrowserModeWindow {
		if resp, err := cdpHTTP.Get(b.httpBase() + "/json/activate/" + t.ID); err == nil {
			resp.Body.Close()
		}
	}
	return b.connectaWS(t.WS)
}

// SelectTab passa a una altra pestanya (número de tabs o id).
func (b *Browser) SelectTab(sel string) error {
	t, err := b.findTab(sel)
	if err != nil {
		return err
	}
	if err := b.selectTab(t); err != nil {
		return err
	}
	return b.esperaCarrega()
}

// NewTab obre una pestanya nova (amb url, si n'hi ha) i hi passa.
func (b *Browser) NewTab(rawURL string) error {
	target := "about:blank"
	if strings.TrimSpace(rawURL) != "" {
		u, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("browser: cal una URL http(s) completa")
		}
		target = u.String()
	}
	b.mu.Lock()
	b.ax = nil
	b.mu.Unlock()
	if et := b.extT(); et != nil {
		nt, id, err := et.hub.open(b.ctx(), target)
		if err != nil {
			return err
		}
		b.t = nt
		b.tabID = strconv.Itoa(id)
		if err := b.prepara(); err != nil {
			return err
		}
		return b.esperaCarrega()
	}
	if b.port == 0 {
		return errNoTabs
	}
	req, _ := http.NewRequest("PUT", b.httpBase()+"/json/new?"+target, nil)
	resp, err := cdpHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("browser: no puc obrir pestanya: %v", err)
	}
	var t tabInfo
	json.NewDecoder(resp.Body).Decode(&t)
	resp.Body.Close()
	if t.WS == "" {
		return fmt.Errorf("browser: pestanya nova sense depuració")
	}
	if err := b.connectaWS(t.WS); err != nil {
		return err
	}
	return b.esperaCarrega()
}

// CloseTab tanca una pestanya; si era l'actual, passa a una altra.
func (b *Browser) CloseTab(sel string) (string, error) {
	var t tabInfo
	var err error
	if strings.TrimSpace(sel) == "" {
		t, err = b.findTab(b.tabID)
	} else {
		t, err = b.findTab(sel)
	}
	if err != nil {
		return "", err
	}
	if et := b.extT(); et != nil {
		id, _ := strconv.Atoi(t.ID)
		if err := et.hub.closeTab(b.ctx(), id); err != nil {
			return "", err
		}
	} else {
		resp, err := cdpHTTP.Get(b.httpBase() + "/json/close/" + t.ID)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
	}
	if t.ID != b.tabID {
		return "closed tab " + t.Title, nil
	}
	list, _ := b.tabList()
	if len(list) == 0 {
		if err := b.NewTab(""); err != nil {
			return "", err
		}
		return "closed the last tab; opened a blank one", nil
	}
	if err := b.selectTab(list[0]); err != nil {
		return "", err
	}
	return "closed tab; now on " + b.On(), nil
}
