// Package telegram — models.go: tria de model en viu per xat i rol.
//
// /model llista els models reals de cada provider (GET <base>/models, com
// /api/models del serve) i desa un override per xat+rol a
// telegram-models.json (0600, al costat de sessions/). El TUI no el fa
// servir: és només del bot.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// ModelRef és un model d'un provider.
type ModelRef struct {
	Provider string
	Model    string
	// Unavailable marca un model que el config declara però que la sonda no
	// ha retornat: el provider és caigut (Halogen viu a 127.0.0.1:8731 i és
	// exclusiu del router local, així que només un dels dos respon). El
	// teclat el mostra igualment; el nom que es desa és el net.
	Unavailable bool
}

func (m ModelRef) String() string { return m.Provider + "/" + m.Model }

// noDisponible és el sufix que marca al teclat un model que no ha respost.
const noDisponible = " (no disponible)"

// fetchModelIDs retorna els IDs de GET <base>/models (duplicat mínim del de
// web/serve.go: el bot és un procés separat i no pot cridar el serve).
func fetchModelIDs(base, key string) ([]string, error) {
	return llm.ProbeModels(base, key)
}

// modelsPath és el fitxer d'overrides (al costat de sessions/).
func (b *Bot) modelsPath() string {
	if b.modelsFile != "" {
		return b.modelsFile
	}
	return filepath.Join(filepath.Dir(b.sessionsDir), "telegram-models.json")
}

// loadModelsOv carrega els overrides (best effort).
func (b *Bot) loadModelsOv() {
	raw, err := os.ReadFile(b.modelsPath())
	if err != nil {
		return
	}
	var m map[string]map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	b.modelsMu.Lock()
	b.modelsOv = m
	b.modelsMu.Unlock()
}

// saveModelsOv desa els overrides (0600).
func (b *Bot) saveModelsOv() {
	b.modelsMu.Lock()
	raw, err := json.MarshalIndent(b.modelsOv, "", " ")
	b.modelsMu.Unlock()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(b.modelsPath()), 0o700)
	_ = os.WriteFile(b.modelsPath(), raw, 0o600)
}

// overrideFor retorna l'override "provider/model" del xat+rol (o "").
func (b *Bot) overrideFor(chatID int64, role string) string {
	b.modelsMu.Lock()
	defer b.modelsMu.Unlock()
	return b.modelsOv[strconv.FormatInt(chatID, 10)][role]
}

// setOverride desa (o esborra, si ref és "") l'override del xat+rol.
func (b *Bot) setOverride(chatID int64, role, ref string) {
	b.modelsMu.Lock()
	key := strconv.FormatInt(chatID, 10)
	if b.modelsOv == nil {
		b.modelsOv = map[string]map[string]string{}
	}
	if ref == "" {
		if b.modelsOv[key] != nil {
			delete(b.modelsOv[key], role)
		}
	} else {
		if b.modelsOv[key] == nil {
			b.modelsOv[key] = map[string]string{}
		}
		b.modelsOv[key][role] = ref
	}
	b.modelsMu.Unlock()
	b.saveModelsOv()
}

// roleRefFor és roleRef aplicant l'override del xat (si n'hi ha).
func (b *Bot) roleRefFor(chatID int64, name string) (config.Provider, config.Role) {
	if name == "" {
		name = "chat"
	}
	r, ok := b.cfg.Roles[name]
	if !ok {
		r = b.cfg.Roles["chat"]
	}
	ov := b.overrideFor(chatID, name)
	if ov == "" {
		return b.cfg.Providers[r.Provider], r
	}
	if p, m, ok := splitRef(ov); ok {
		if prov, ok := b.cfg.Providers[p]; ok {
			return prov, config.Role{Provider: p, Model: m, Temperature: r.Temperature,
				MaxTokens: r.MaxTokens, ContextWindow: r.ContextWindow}
		}
	}
	// Override antic (només model): mateix provider, model nou.
	if prov, ok := b.cfg.Providers[r.Provider]; ok {
		r.Model = ov
		return prov, r
	}
	return b.cfg.Providers[r.Provider], r
}

func splitRef(ov string) (string, string, bool) {
	if i := strings.Index(ov, "/"); i > 0 {
		return ov[:i], ov[i+1:], true
	}
	return "", "", false
}

// refreshModels retorna el llistat viu (cau de 5 minuts).
func (b *Bot) refreshModels(ctx context.Context) []ModelRef {
	b.modelsMu.Lock()
	if time.Since(b.modelsAt) < 5*time.Minute && len(b.modelsCache) > 0 {
		out := b.modelsCache
		b.modelsMu.Unlock()
		return out
	}
	b.modelsMu.Unlock()

	type prov struct{ name, base, key string }
	var provs []prov
	for n, p := range b.cfg.Providers {
		provs = append(provs, prov{n, p.BaseURL, p.APIKey})
	}
	sort.Slice(provs, func(i, j int) bool { return provs[i].name < provs[j].name })

	var mu sync.Mutex
	var out []ModelRef
	var wg sync.WaitGroup
	for _, pr := range provs {
		wg.Add(1)
		go func(pr prov) {
			defer wg.Done()
			ids, err := fetchModelIDs(pr.base, pr.key)
			if err != nil {
				return
			}
			mu.Lock()
			for _, id := range ids {
				out = append(out, ModelRef{Provider: pr.name, Model: id})
			}
			mu.Unlock()
		}(pr)
	}
	wg.Wait()
	out = b.mergeConfigurats(out)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Model < out[j].Model
	})

	b.modelsMu.Lock()
	b.modelsCache, b.modelsAt = out, time.Now()
	b.modelsMu.Unlock()
	return out
}

// mergeConfigurats afegeix a la llista sondejada els models que el config
// declara als rols i que cap provider ha retornat, marcats com a no
// disponibles: així un provider caigut no amaga del teclat el model que
// l'usuari va configurar, i triar-lo continua sent possible (es desa el nom
// net, sense el sufix). Els que sí que han respost no es dupliquen.
func (b *Bot) mergeConfigurats(sondejats []ModelRef) []ModelRef {
	if b.cfg == nil {
		return sondejats
	}
	vist := make(map[string]bool, len(sondejats))
	for _, r := range sondejats {
		vist[r.String()] = true
	}
	// Els rols, ordenats: la llista ha de ser estable entre execucions.
	noms := make([]string, 0, len(b.cfg.Roles))
	for n := range b.cfg.Roles {
		noms = append(noms, n)
	}
	sort.Strings(noms)
	out := sondejats
	for _, n := range noms {
		r := b.cfg.Roles[n]
		if strings.TrimSpace(r.Provider) == "" || strings.TrimSpace(r.Model) == "" {
			continue
		}
		if _, ok := b.cfg.Providers[r.Provider]; !ok {
			continue // sense endpoint declarat no s'hi pot apuntar
		}
		ref := ModelRef{Provider: r.Provider, Model: r.Model, Unavailable: true}
		if vist[ref.String()] {
			continue
		}
		vist[ref.String()] = true
		out = append(out, ref)
	}
	return out
}

// currentRef descriu el model actiu del xat+rol ("provider/model").
func (b *Bot) currentRef(chatID int64, role string) string {
	if ov := b.overrideFor(chatID, role); ov != "" {
		return ov + b.tr(" (triat)", " (selected)")
	}
	if r, ok := b.cfg.Roles[role]; ok {
		return r.Provider + "/" + r.Model
	}
	return "?"
}

// shortModel retalla "provider/model" per al peu de sessió.
func shortModel(ref string) string {
	ref = strings.TrimSuffix(ref, " (triat)")
	ref = strings.TrimSuffix(ref, " (selected)")
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// modelList mostra els models vius agrupats per provider (amb botons).
func (b *Bot) modelList(ctx context.Context, chatID int64, st *chatState) {
	st.mu.Lock()
	role := st.role
	st.mu.Unlock()
	refs := b.refreshModels(ctx)
	if len(refs) == 0 {
		b.send(ctx, chatID, b.tr("⚠️ Cap provider respon al llistat de models (/v1/models). Revisa que el router local (:8087) estigui en marxa.", "⚠️ No provider returned a model list (/v1/models). Check that the local router (:8087) is running."))
		return
	}
	cur := b.currentRef(chatID, role)
	var lines []string
	lines = append(lines, fmt.Sprintf("🧠 <b>"+b.tr("Models", "Models")+"</b> · "+b.tr("rol", "role")+" <code>%s</code> · "+b.tr("actual", "current")+" <code>%s</code>", esc(role), esc(cur)))
	lines = append(lines, b.tr("Toca per fer-lo servir en aquest xat (es desa).", "Tap a model to use it in this chat (selection is saved)."))
	if teNoDisponible(refs) {
		lines = append(lines, b.tr("Els marcats amb <i>"+esc(noDisponible)+"</i> no han respost ara mateix; pots triar-los igualment i es desaran.", "Models marked <i>"+esc(b.unavailableLabel())+"</i> did not respond; you can still select and save them."))
	}
	b.sendKB(ctx, chatID, strings.Join(lines, "\n"), b.modelKB(chatID, role, refs, nil))
}

// teNoDisponible diu si la llista duu algun model que no ha respost.
func teNoDisponible(refs []ModelRef) bool {
	for _, r := range refs {
		if r.Unavailable {
			return true
		}
	}
	return false
}

func (b *Bot) unavailableLabel() string {
	return b.tr(noDisponible, " (unavailable)")
}

// modelKB construeix el teclat (només els índexs `only`, o tots).
func (b *Bot) modelKB(chatID int64, role string, refs []ModelRef, only map[int]bool) Keyboard {
	cur := b.currentRef(chatID, role)
	cur = strings.TrimSuffix(cur, b.tr(" (triat)", " (selected)"))
	var kb Keyboard
	var lastProv string
	var row []InlineButton
	flush := func() {
		if len(row) > 0 {
			kb = append(kb, row)
			row = nil
		}
	}
	for i, ref := range refs {
		if only != nil && !only[i] {
			continue
		}
		if ref.Provider != lastProv {
			flush()
			lastProv = ref.Provider
		}
		label := truncate(ref.Model, 28)
		if ref.Unavailable {
			// El sufix no s'ha de perdre pel retall: es retalla el nom, no
			// la marca (si no, un model llarg no es distingiria d'un viu).
			unavailable := b.unavailableLabel()
			label = truncate(ref.Model, 28-len(unavailable)) + unavailable
		}
		if ref.String() == cur {
			label = "● " + label
		}
		row = append(row, InlineButton{Text: label, Data: "m:" + itoa(i)})
		if len(row) == 2 {
			flush()
		}
		if len(kb) >= 14 {
			break
		}
	}
	flush()
	kb = append(kb, []InlineButton{
		{Text: "↺ " + b.tr("Defecte", "Default"), Data: "m:clear"},
		{Text: "↻ " + b.tr("Actualitza", "Refresh"), Data: "m:list"},
	})
	return kb
}

// modelTap resol el botó "m:<índex>" del llistat en cau.
func (b *Bot) modelTap(ctx context.Context, chatID int64, st *chatState, data string) {
	b.modelsMu.Lock()
	refs := b.modelsCache
	b.modelsMu.Unlock()
	if len(refs) == 0 {
		b.send(ctx, chatID, b.tr("llista caducada · /model de nou", "model list expired · use /model again"))
		return
	}
	n, err := strconv.Atoi(strings.TrimPrefix(data, "m:"))
	if err != nil || n < 0 || n >= len(refs) {
		b.send(ctx, chatID, b.tr("llista caducada · /model de nou", "model list expired · use /model again"))
		return
	}
	st.mu.Lock()
	role := st.role
	st.mu.Unlock()
	b.applyModel(ctx, chatID, role, refs[n])
}

// modelPick tria per text lliure (/model <nom>).
func (b *Bot) modelPick(ctx context.Context, chatID int64, st *chatState, text string) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "clear" || text == "defecte" || text == "-" {
		st.mu.Lock()
		role := st.role
		st.mu.Unlock()
		b.setOverride(chatID, role, "")
		b.sendKB(ctx, chatID, "↺ "+b.tr("Model per defecte del rol", "Role default model")+"\n"+b.roleText(chatID, st), b.roleKB(role))
		return
	}
	refs := b.refreshModels(ctx)
	var hits []int
	for i, ref := range refs {
		if strings.Contains(strings.ToLower(ref.String()), text) {
			hits = append(hits, i)
		}
	}
	st.mu.Lock()
	role := st.role
	st.mu.Unlock()
	switch len(hits) {
	case 0:
		b.send(ctx, chatID, b.tr("cap model coincideix amb <code>", "no models match <code>")+esc(text)+"</code> · /model "+b.tr("per veure'ls", "to list them"))
	case 1:
		b.applyModel(ctx, chatID, role, refs[hits[0]])
	default:
		only := map[int]bool{}
		for _, h := range hits {
			only[h] = true
		}
		b.sendKB(ctx, chatID, fmt.Sprintf(b.tr("%d models coincideixen amb <code>%s</code> · toca'n un (rol <code>%s</code>)", "%d models match <code>%s</code> · tap one to select it (role <code>%s</code>)"),
			len(hits), esc(text), esc(role)), b.modelKB(chatID, role, refs, only))
	}
}

// applyModel desa l'override i confirma (amb endpoint visible).
func (b *Bot) applyModel(ctx context.Context, chatID int64, role string, ref ModelRef) {
	b.setOverride(chatID, role, ref.String())
	base := ""
	if p, ok := b.cfg.Providers[ref.Provider]; ok {
		base = p.BaseURL
	}
	b.sendKB(ctx, chatID, fmt.Sprintf("🧠 "+b.tr("Rol", "Role")+" <code>%s</code> → <code>%s</code>\n"+b.tr("lloc", "endpoint")+" <code>%s</code>\n"+b.tr("Es desa per aquest xat (/model clear per desfer-ho).", "Saved for this chat (use /model clear to restore the default)."),
		esc(role), esc(ref.String()), esc(base)), b.roleKB(role))
}
