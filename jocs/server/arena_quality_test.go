package server

import "testing"

func TestArenaQualityLadderAndVersions(t *testing.T) {
	mk := func(id string, a, b string, winner int, mdA, mdB string, at int64) *arenaMatch {
		return &arenaMatch{ID: id, Game: "nfl", Status: "final", Owners: [2]string{a, b}, Winner: winner, MD: [2]string{mdA, mdB}, Started: at, Finished: at + 1}
	}
	v1 := "# v1\nPlay aggressive."
	v2 := "# v2\n1. 3rd & 7+: pass_medium.\n2. Otherwise: run_inside."
	finals := []*arenaMatch{
		mk("m1", "ana", arenaVagueBot, 1, v1, "rk", 10),           // v1 loses to the vague bot
		mk("m2", "ana", arenaVagueBot, 0, v2, "rk", 20),           // v2 beats it
		mk("m3", "ana", "bot:analyst", 1, v2, "an", 30),           // and loses to the expert
		mk("m4", "bot:analyst", arenaVagueBot, 0, "an", "rk", 40), // bot vs bot: the expert wins
		mk("m5", arenaVagueBot, "bot:turtle", 1, "rk", "tt", 50),  // the vague bot loses again
	}
	coach := map[string][2]int{"m1": {10, 0}, "m2": {20, 0}}
	players := map[string]*arenaPlayer{"ana": {Owner: "ana", Games: map[string]*arenaPlayerGame{}}}

	sigs := map[string]arenaMDSigs{}
	for _, m := range finals {
		sigs[m.ID] = arenaMDSigs{mdSig(m.MD[0]), mdSig(m.MD[1])}
	}
	q := arenaQualityOf(finals, coach, sigs, players)

	// Structured prompts against Bot Rookie: m1 (lost), m2 (won), m4 (won), m5 (won).
	if q.VsVague.Played != 4 || q.VsVague.Won != 3 {
		t.Fatalf("vs vague = %+v, want 3/4", q.VsVague)
	}
	if q.People.Played != 2 || q.People.Won != 1 {
		t.Fatalf("people = %+v, want 1/2", q.People)
	}
	var analyst *arenaRung
	for i := range q.Rungs {
		if q.Rungs[i].Owner == "bot:analyst" {
			analyst = &q.Rungs[i]
		}
	}
	if analyst == nil || analyst.Overall.Played != 2 || analyst.Overall.Won != 2 || analyst.VsVague.Won != 1 {
		t.Fatalf("analyst rung = %+v", analyst)
	}

	p := players["ana"]
	if p.VsBots["rookie"].Played != 2 || p.VsBots["rookie"].Won != 1 || p.VsBots["expert"].Played != 1 {
		t.Fatalf("vs bots = %+v %+v", p.VsBots["rookie"], p.VsBots["expert"])
	}
	vs := p.Games["nfl"].Versions
	if len(vs) != 2 || vs[0].N != 1 || vs[0].Won != 0 || vs[1].N != 2 || vs[1].Played != 2 || vs[1].Won != 1 {
		t.Fatalf("versions = %+v", vs)
	}
	if vs[0].Coach != 40 || vs[1].Coach != 80 || vs[1].Lines != 3 {
		t.Fatalf("coach/lines = %+v", vs)
	}
}
