package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/jocs/football"
)

// Sense membres, el magatzem en crea el joc per defecte i el desa; si n'hi
// ha de propis, mana el que hi ha. La lliga es desa i es pot esborrar sense
// perdre els membres.
func TestFileTeamStore(t *testing.T) {
	dir := t.TempDir()
	var st TeamStore = newFileTeamStore(dir)
	ms, err := st.Members()
	if err != nil || len(ms) != 3 || ms[0].ID != "tramuntana" {
		t.Fatalf("per defecte: %+v %v", ms, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "members.json")); err != nil {
		t.Fatal("els membres per defecte s'han de desar")
	}
	if lg, err := st.League(); lg != nil || err != nil {
		t.Fatalf("encara no hi ha lliga: %v %v", lg, err)
	}

	own := []Member{{ID: "a", Name: "Aire"}, {ID: "b", Name: "Brisa"}}
	if err := writeJSONFile(filepath.Join(dir, "members.json"), membersFile{Version: 1, Members: own}); err != nil {
		t.Fatal(err)
	}
	ms, _ = st.Members()
	if len(ms) != 2 || ms[1].Name != "Brisa" {
		t.Fatalf("membres propis: %+v", ms)
	}

	lg := football.NewLeague(leagueStars(ms), "en", 7)
	if err := st.SaveLeague(lg); err != nil {
		t.Fatal(err)
	}
	back, err := st.League()
	if err != nil || back == nil || back.Teams[0].Coach != lg.Teams[0].Coach {
		t.Fatalf("lliga desada: %+v %v", back, err)
	}
	if err := st.ResetLeague(); err != nil {
		t.Fatal(err)
	}
	if back, _ := st.League(); back != nil {
		t.Fatal("reset ha d'esborrar la lliga")
	}
	if ms, _ := st.Members(); len(ms) != 2 {
		t.Fatal("reset no toca els membres")
	}
	if got := (Member{Name: "Aire", Role: "coach", Persona: "Calm."}).identity(); got != "You are Aire (coach). Calm." {
		t.Fatalf("identity: %q", got)
	}
}

// El cos tècnic: cada posició decideix els seus comandaments del pla, i el
// model ho llegeix amb les instruccions desades (o l'exemple, si no n'hi ha).
func TestStaffBriefAndSave(t *testing.T) {
	brief := staffBrief(map[string]string{"qb": "Throw to the tight end on third down."})
	for _, want := range []string{"Quarterback (decides focus): Throw to the tight end on third down.", "Defensive coordinator (decides blitz)", "Head coach (decides aggression, tempo)"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief sense %q:\n%s", want, brief)
		}
	}
	st := newFileTeamStore(t.TempDir())
	ms, _ := st.Members()
	ms[0].Staff = map[string]string{"dc": "Blitz every third down."}
	if err := st.SaveMembers(ms); err != nil {
		t.Fatal(err)
	}
	back, _ := st.Members()
	if back[0].Staff["dc"] != "Blitz every third down." {
		t.Fatalf("el cos tècnic s'ha de desar: %+v", back[0])
	}
}
