package server

import (
	"strings"
	"testing"
)

func TestCleanTeamList(t *testing.T) {
	got := cleanTeamList([]string{" Data ", "data", "", "Cloud", strings.Repeat("x", 60)})
	if len(got) != 3 || got[0] != "Data" || got[1] != "Cloud" || len([]rune(got[2])) != arenaTeamNameMax {
		t.Fatalf("clean = %q", got)
	}
}

func TestArenaTeamTableRanksByAverageXP(t *testing.T) {
	ana := &arenaPlayer{Owner: "ana", Display: "Ana", Played: 4, Reviewed: 3, Prompt: 70, BotsBeaten: 2, CoachFirst: 12, CoachLast: 18,
		VsBots: map[string]*arenaWL{"rookie": {Played: 2, Won: 2}}}
	joan := &arenaPlayer{Owner: "joan", Display: "Joan", Played: 1, Reviewed: 1, Prompt: 40}
	pere := &arenaPlayer{Owner: "pere", Display: "Pere"} // has not played
	maria := &arenaPlayer{Owner: "maria", Display: "Maria", Played: 2, Reviewed: 2, Prompt: 60, Official: [3]int{1, 0, 0}, CoachFirst: 10, CoachLast: 15}
	cfg := arenaTeamsCfg{List: []string{"Cloud", "Data", "Empty"}, Members: map[string]string{"ana": "Data", "joan": "Data", "pere": "Cloud", "maria": "Cloud", "ghost": "Data"}}

	rows := arenaTeamTable([]*arenaPlayer{ana, joan, pere, maria}, cfg)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	// Data: ana 30+40+140=210, joan 10+80=90 → 150. Cloud: maria 20+30+120=170, pere 0 → 85.
	if rows[0].Name != "Data" || rows[0].XP != 150 || rows[0].Members != 2 || rows[0].Active != 2 || rows[0].Top != "Ana" {
		t.Fatalf("first = %+v", rows[0])
	}
	if rows[1].Name != "Cloud" || rows[1].XP != 85 || rows[1].Active != 1 || rows[1].Official != 1 || rows[1].Improvement != 20 {
		t.Fatalf("second = %+v", rows[1])
	}
	if rows[0].VsVague.Won != 2 || rows[0].Prompt != 55 || rows[0].Improvement != 24 {
		t.Fatalf("data details = %+v", rows[0])
	}
	if rows[2].Name != "Empty" || rows[2].Members != 0 || rows[2].Rank != 3 {
		t.Fatalf("empty team = %+v", rows[2])
	}
	if ana.Team != "Data" || pere.Team != "Cloud" {
		t.Fatalf("player teams: %q %q", ana.Team, pere.Team)
	}
	if arenaTeamTable([]*arenaPlayer{ana}, arenaTeamsCfg{}) != nil {
		t.Fatal("no list = no team mode")
	}
}

func TestArenaReportRowsAndCSVSafety(t *testing.T) {
	p := &arenaPlayer{Owner: "ana", Display: "=SUM(A1)", Team: "Data", Played: 3, Won: 2, Lost: 1, Reviewed: 2, Prompt: 64,
		CoachFirst: 12, CoachLast: 18, Principles: map[string]float64{"clarity": 4, "specificity": 3.5},
		VsBots: map[string]*arenaWL{"rookie": {Played: 2, Won: 1}},
		Games:  map[string]*arenaPlayerGame{"nfl": {Versions: []arenaMDVersion{{N: 1, Played: 2, Won: 0}, {N: 3, Played: 1, Won: 1}}}}}
	rows := arenaReportRows([]*arenaPlayer{p})
	if len(rows) != 2 || len(rows[1]) != len(arenaReportHeader) {
		t.Fatalf("rows = %v", rows)
	}
	col := func(name string) string {
		for i, h := range arenaReportHeader {
			if h == name {
				return rows[1][i]
			}
		}
		t.Fatalf("no column %s", name)
		return ""
	}
	if col("first_coach") != "48.0" || col("latest_coach") != "72.0" || col("coach_gain") != "24.0" || col("clarity") != "4.0" {
		t.Fatalf("coach columns: %v", rows[1])
	}
	if col("md_versions") != "3" || col("first_version_win_rate") != "0" || col("latest_version_win_rate") != "100" || col("vs_vague_won") != "1" {
		t.Fatalf("version columns: %v", rows[1])
	}
	if csvSafe("=SUM(A1)") != "'=SUM(A1)" || csvSafe("-12.5") != "-12.5" || csvSafe("Ana") != "Ana" {
		t.Fatal("csvSafe")
	}
}
