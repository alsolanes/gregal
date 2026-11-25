package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
)

// «El que segueixes» a «Avui»: els equips (o qualsevol altra cosa) que la
// persona segueix. Cada seguiment té una rutina diària de només lectura del
// seu vent (Gregal si no en té cap) que busca a la web el proper partit o la
// propera novetat i respon un JSON petit; «Avui» d'una ullada ensenya l'última
// resposta bona. Res d'això avisa ningú pel seu compte.
//
//	POST /api/winds/avui/settings {follow:{kind,name,wind?}} → l'afegeix (o l'actualitza)
//	POST /api/winds/avui/settings {unfollow:"<id>"}           → el treu
//	eina routine {wind, action: follow|unfollow, name, kind?}

const (
	avuiFollowsFile = "winds-avui-follows.json"
	followJobPrefix = "follow-"
	maxFollows      = 12
)

type avuiFollow struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	Wind string `json:"wind,omitempty"`
}

// followNews és l'última resposta bona de la rutina d'un seguiment. None: la
// cerca no ha trobat res; Error: per què ha fallat l'última mirada (la resposta
// bona d'abans es queda).
type followNews struct {
	Title   string `json:"title,omitempty"`
	When    string `json:"when,omitempty"`
	Detail  string `json:"detail,omitempty"`
	URL     string `json:"url,omitempty"`
	Updated int64  `json:"updated,omitempty"`
	None    bool   `json:"none,omitempty"`
	Error   string `json:"error,omitempty"`
}

type glanceFollow struct {
	avuiFollow
	followNews
}

func avuiFollowsPath(s *Server) string { return filepath.Join(s.sessDir(), avuiFollowsFile) }

func (s *Server) loadFollowNews() map[string]followNews {
	out := map[string]followNews{}
	if raw, err := os.ReadFile(avuiFollowsPath(s)); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func (s *Server) saveFollowNews(id string, n followNews) error {
	all := s.loadFollowNews()
	all[id] = n
	raw, err := json.Marshal(all)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(avuiFollowsPath(s)), 0o700)
	return writeFileAtomic(avuiFollowsPath(s), raw)
}

func (s *Server) glanceFollows(st avuiSettings) []glanceFollow {
	news := s.loadFollowNews()
	out := make([]glanceFollow, 0, len(st.Follows))
	for _, f := range st.Follows {
		out = append(out, glanceFollow{avuiFollow: f, followNews: news[f.ID]})
	}
	return out
}

// addFollow afegeix un seguiment; si ja hi era (mateix nom), l'actualitza.
func (s *Server) addFollow(st *avuiSettings, f avuiFollow) (avuiFollow, error) {
	f.Name = oneLine(f.Name, 80)
	f.ID = companion.SlugWind(f.Name)
	if f.ID == "" {
		return f, errors.New(s.tr("digues què vols seguir", "say what to follow"))
	}
	if f.Kind != "team" {
		f.Kind = "other"
	}
	if ref := strings.TrimSpace(f.Wind); ref != "" {
		w, ok := windByRef(s, ref)
		if !ok {
			return f, errors.New(s.tr("vent desconegut", "unknown wind"))
		}
		f.Wind = w.ID
	}
	for i := range st.Follows {
		if st.Follows[i].ID == f.ID {
			st.Follows[i] = f
			return f, nil
		}
	}
	if len(st.Follows) >= maxFollows {
		return f, fmt.Errorf(s.tr("com a molt %d coses seguides", "at most %d things followed"), maxFollows)
	}
	st.Follows = append(st.Follows, f)
	return f, nil
}

func removeFollow(st *avuiSettings, ref string) (avuiFollow, bool) {
	id := companion.SlugWind(ref)
	for i, f := range st.Follows {
		if f.ID == id {
			st.Follows = append(st.Follows[:i:i], st.Follows[i+1:]...)
			return f, true
		}
	}
	return avuiFollow{}, false
}

func (s *Server) applyFollowReq(st *avuiSettings, req avuiSettingsReq) error {
	if req.Follow != nil {
		if _, err := s.addFollow(st, *req.Follow); err != nil {
			return err
		}
	}
	if req.Unfollow != "" {
		if _, ok := removeFollow(st, req.Unfollow); !ok {
			return errors.New(s.tr("no segueixes res amb aquest nom", "you follow nothing with that name"))
		}
	}
	return nil
}

func followJobID(owner, id string) string {
	if owner == "" {
		owner = "me"
	}
	return followJobPrefix + strings.NewReplacer("/", "-", " ", "-").Replace(owner) + "-" + id
}

func isFollowJob(id string) bool { return strings.HasPrefix(id, followJobPrefix) }

// followWind: el vent del seguiment o, si no en té (o ja no hi és), Gregal.
func (s *Server) followWind(f avuiFollow) (companion.Wind, bool) {
	if f.Wind != "" {
		if w, ok := windByRef(s, f.Wind); ok {
			return w, true
		}
	}
	return s.ensureGregalWind()
}

// syncFollowJobs deixa una rutina diària per seguiment d'aquest compte, i cap més.
func (s *Server) syncFollowJobs(st avuiSettings) error {
	if s.jobs == nil {
		return nil
	}
	list, err := s.jobs.List()
	if err != nil {
		return err
	}
	owner := s.localOwner()
	want := map[string]avuiFollow{}
	for _, f := range st.Follows {
		want[followJobID(owner, f.ID)] = f
	}
	have := map[string]string{}
	for _, j := range list {
		if !isFollowJob(j.ID) || j.WindOwner != owner {
			continue
		}
		if _, ok := want[j.ID]; !ok {
			if err := s.jobs.Delete(j.ID); err != nil {
				return err
			}
			continue
		}
		have[j.ID] = j.Wind
	}
	for id, f := range want {
		w, ok := s.followWind(f)
		if !ok {
			return errors.New(s.tr("primer crea un vent", "create a wind first"))
		}
		if have[id] == w.ID {
			continue
		}
		// Sense LastAt: la primera mirada es fa de seguida, no demà.
		if _, err := s.jobs.Save(jobs.Job{
			ID: id, Name: s.tr("Segueixo", "Following") + " · " + f.Name, Prompt: "Keep up to date on " + f.Name,
			Kind: "daily", Time: "07:00", Enabled: true, Wind: w.ID, WindOwner: owner,
		}); err != nil {
			return err
		}
	}
	return nil
}

// followTool és l'acció follow|unfollow de l'eina routine («segueix el Barça»).
func (s *Server) followTool(w companion.Wind, a routineArgs) (string, error) {
	st := s.avuiSettings()
	var msg string
	if strings.EqualFold(strings.TrimSpace(a.Action), "unfollow") {
		f, ok := removeFollow(&st, a.Name)
		if !ok {
			return "", fmt.Errorf("no segueixes res amb el nom %q", a.Name)
		}
		msg = "Ja no segueixes «" + f.Name + "»."
	} else {
		f, err := s.addFollow(&st, avuiFollow{Kind: a.Kind, Name: a.Name, Wind: w.ID})
		if err != nil {
			return "", err
		}
		msg = "Ara segueixes «" + f.Name + "»: cada dia en buscaràs la propera novetat i sortirà a Avui."
	}
	if err := s.saveAvuiSettings(st); err != nil {
		return "", err
	}
	if err := s.syncFollowJobs(st); err != nil {
		return "", err
	}
	return msg, nil
}

// followLoop és la cerca del vent; els tests la substitueixen.
var followLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, 8, p, role)
}

func (s *Server) followBrief(w companion.Wind, f avuiFollow, now time.Time) string {
	what := "something the user follows"
	look := "the next thing coming up about it (a date, a release, an event) or, if nothing is scheduled, the latest news"
	if f.Kind == "team" {
		what = "a sports team the user follows"
		look = "the team's next match: opponent, day and kick-off time in the user's local time, and the competition"
	}
	return companion.WindIdentity(w) +
		fmt.Sprintf("\n\nYOU KEEP THE USER UP TO DATE on %s: %s.\nIt is now %s (%s).\n", what, f.Name, now.Format("Monday 2 January 2006, 15:04"), now.Location().String()) +
		"Use web_search and web_fetch to find " + look + ". Only facts you read on a page; never invent a date or a URL.\n" +
		`ANSWER with only one JSON object and nothing else: {"title": "short, e.g. Barça – Madrid", "when": "RFC 3339 date-time with the offset, or empty if there is no date", "detail": "one short line, e.g. the competition", "url": "the page where you read it"}. Write title and detail in the user's language. If you find nothing coming up, answer {"none": true}.` +
		"\n\n" + config.LanguageDirective(s.cfg.Lang())
}

// parseFollowAnswer llegeix el JSON del vent; ok=false si no és el que toca.
func parseFollowAnswer(out string) (followNews, bool) {
	var n followNews
	i, j := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if i < 0 || j < i || json.Unmarshal([]byte(out[i:j+1]), &n) != nil {
		return n, false
	}
	if n.None {
		return followNews{None: true}, true
	}
	n.Title, n.Detail, n.URL, n.When = oneLine(n.Title, 110), oneLine(n.Detail, 200), strings.TrimSpace(n.URL), strings.TrimSpace(n.When)
	if n.Title == "" {
		return n, false
	}
	if _, err := time.Parse(time.RFC3339, n.When); n.When != "" && err != nil {
		return n, false
	}
	if n.URL != "" && !strings.HasPrefix(n.URL, "https://") && !strings.HasPrefix(n.URL, "http://") {
		return n, false
	}
	return n, true
}

// executeFollow és el pas de la rutina d'un seguiment.
func (s *Server) executeFollow(ctx context.Context, job jobs.Job) (jobs.Run, error) {
	start := time.Now()
	run := jobs.Run{Wind: job.Wind, WindOwner: job.WindOwner, JobID: job.ID, JobName: job.Name, StartedAt: start}
	finish := func(status, output, errMsg string) (jobs.Run, error) {
		run.FinishedAt, run.Status, run.Output, run.Error = time.Now(), status, output, errMsg
		_ = s.jobs.AddRun(run)
		_ = s.jobs.MarkRun(job.ID, run.FinishedAt, status)
		if errMsg != "" {
			return run, errors.New(errMsg)
		}
		return run, nil
	}
	var f avuiFollow
	for _, x := range s.avuiSettings().Follows {
		if followJobID(s.localOwner(), x.ID) == job.ID {
			f = x
		}
	}
	if f.ID == "" {
		return finish("ok", "", "")
	}
	// fail deixa a «Avui» per què no hi ha novetat; la resposta bona d'abans es queda.
	fail := func(errMsg, shown string) (jobs.Run, error) {
		n := s.loadFollowNews()[f.ID]
		n.Error = shown
		if err := s.saveFollowNews(f.ID, n); err != nil {
			errMsg += "; " + err.Error()
		}
		return finish("error", "", errMsg)
	}
	w, ok := s.followWind(f)
	if !ok {
		return fail("the wind for this follow is not available", s.tr("El vent que ho seguia ja no hi és.", "The wind that followed it is gone."))
	}
	res, err := followLoop(s, ctx, s.followBrief(w, f, start.In(s.loadConnections().zone())))
	if err != nil {
		msg := friendlyRunError(err.Error())
		return fail(msg, s.tr("No ho he pogut buscar: ", "Could not look it up: ")+msg)
	}
	n, ok := parseFollowAnswer(res.text)
	if !ok {
		return fail("the answer was not the expected JSON; the previous one stays",
			s.tr("La cerca no ha donat una resposta clara; ho tornaré a provar.", "The search gave no clear answer; I will try again."))
	}
	n.Updated = start.Unix()
	if err := s.saveFollowNews(f.ID, n); err != nil {
		return finish("error", "", err.Error())
	}
	return finish("ok", n.Title, "")
}
