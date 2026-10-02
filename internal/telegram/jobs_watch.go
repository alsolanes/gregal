package telegram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/jobs"
)

// watchJobs avisa els usuaris permesos quan un job amb Notify acaba.
// El servidor i el bot són processos separats: el punt de trobada és el
// disc (mateix ~/.local/share/gregal/jobs) amb marca d'aigua.
func (b *Bot) watchJobs(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.notifyNewRuns(ctx)
		}
	}
}

func (b *Bot) notifyNewRuns(ctx context.Context) {
	st, err := jobs.New("")
	if err != nil {
		return
	}
	runs, err := st.Runs("", 10)
	if err != nil {
		return
	}
	marked := readWatermark()
	fresh := []jobs.Run{}
	for _, r := range runs {
		if r.FinishedAt.IsZero() {
			continue
		}
		if _, ok := marked[r.ID]; !ok {
			fresh = append(fresh, r)
		}
	}
	if len(fresh) == 0 {
		return
	}
	// Marca abans d'enviar: si Telegram falla, no reintentem cada minut.
	for _, r := range fresh {
		marked[r.ID] = true
	}
	writeWatermark(marked)
	b.mu.Lock()
	owners := make([]int64, 0, len(b.allowed))
	for id := range b.allowed {
		owners = append(owners, id)
	}
	b.mu.Unlock()
	for _, r := range fresh {
		if !notifyForRun(st, r) {
			continue
		}
		text := "📰 <b>" + esc(b.jobRunTitle(r)) + "</b>\n" + esc(runExcerpt(r)) +
			"\n\n" + b.tr("Mira-ho al panell de Jobs de l'app.", "Open it in the app's Jobs panel.")
		for _, id := range owners {
			if _, err := b.api.SendMessage(ctx, id, text, nil); err != nil {
				b.logf("avís job: %v", err)
			}
		}
	}
}

// notifyForRun respecta el flag Notify del job (per defecte avisa).
func notifyForRun(st *jobs.Store, r jobs.Run) bool {
	list, err := st.List()
	if err != nil {
		return true
	}
	for _, j := range list {
		if j.ID == r.JobID {
			return j.Notify
		}
	}
	return true // job esborrat: avisa igualment
}

func (b *Bot) jobRunTitle(r jobs.Run) string {
	lang := "en"
	if b.cfg != nil {
		lang = b.cfg.Lang()
	}
	return runTitleFor(r, lang)
}

func runTitleFor(r jobs.Run, lang string) string {
	if r.Status == "ok" {
		if lang == "ca" {
			return r.JobName + " llest"
		}
		return r.JobName + " completed"
	}
	if lang == "ca" {
		return r.JobName + " ha fallat"
	}
	return r.JobName + " failed"
}

func runExcerpt(r jobs.Run) string {
	if r.Status != "ok" {
		return excerptOf(r.Error, 200)
	}
	return excerptOf(markdownToText(r.Output), 300)
}

// excerptOf talla text net (sense marques) a n runes.
func excerptOf(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

// markdownToText neteja marques per a l'extracte de l'avís (el ric queda a l'app).
func markdownToText(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		for strings.HasPrefix(t, "#") {
			t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		}
		t = strings.TrimPrefix(t, ">")
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "-"), "*"))
		if t == "" || t == "---" {
			continue
		}
		out = append(out, t)
		if len(out) >= 6 {
			break
		}
	}
	return strings.Join(out, "\n")
}

func watermarkPath() string { return filepath.Join(jobs.DefaultDir(), ".notified") }

func readWatermark() map[string]bool {
	m := map[string]bool{}
	raw, err := os.ReadFile(watermarkPath())
	if err != nil {
		return m
	}
	for _, ln := range strings.Split(string(raw), "\n") {
		if id := strings.TrimSpace(ln); id != "" {
			m[id] = true
		}
	}
	return m
}

func writeWatermark(m map[string]bool) {
	var b strings.Builder
	n := 0
	for id := range m {
		b.WriteString(id + "\n")
		n++
		if n >= 500 {
			break
		}
	}
	_ = os.WriteFile(watermarkPath(), []byte(b.String()), 0o600)
}
