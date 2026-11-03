package cs2

import "gregal/internal/arena"

var pbStrats = map[string]arena.PlayOption{
	"rush_a":      {Label: "Rush A", Plain: "All five run into A fast. Floods an empty site, gets soaked by a stacked one."},
	"rush_b":      {Label: "Rush B", Plain: "All five run into B fast. Floods an empty site, gets soaked by a stacked one."},
	"execute_a":   {Label: "Execute A", Plain: "Smoke and flash A, then take it together. Beats a standard setup and a team that gives the site away to retake it."},
	"execute_b":   {Label: "Execute B", Plain: "Smoke and flash B, then take it together. Beats a standard setup and a team that gives the site away to retake it."},
	"split_a":     {Label: "Split A", Plain: "Hit A from two sides at once. Good against a standard setup."},
	"split_b":     {Label: "Split B", Plain: "Hit B from two sides at once. Good against a standard setup."},
	"default_mid": {Label: "Default (mid)", Plain: "Spread out, win mid and hit the weak site. Beats stacks, loses to aggressive mid."},
	"fake_a_go_b": {Label: "Fake A, go B", Plain: "Make noise at A, then hit B. Fools a team that stacks A or rotates early."},
	"fake_b_go_a": {Label: "Fake B, go A", Plain: "Make noise at B, then hit A. Fools a team that stacks B or rotates early."},
}

var pbSetups = map[string]arena.PlayOption{
	"standard_2_1_2": {Label: "Standard 2-1-2", Plain: "Two on A, one mid, two on B. The safest setup: never loses badly, but fakes and splits beat it."},
	"stack_a":        {Label: "Stack A", Plain: "Most CTs on A. Wins big if they hit A, loses big if they go B."},
	"stack_b":        {Label: "Stack B", Plain: "Most CTs on B. Wins big if they hit B, loses big if they go A."},
	"aggressive_mid": {Label: "Aggressive mid", Plain: "Push mid early for quick splashes. Beats slow defaults and executes, but rushes run past it."},
	"retake_b":       {Label: "Retake B", Plain: "Give B away and win it back after the plant. Beats rushes."},
	"retake_a":       {Label: "Retake A", Plain: "Give A away and win it back after the plant. Beats rushes."},
}

func pbOpts(ids []string, m map[string]arena.PlayOption) []arena.PlayOption {
	out := make([]arena.PlayOption, len(ids))
	for i, id := range ids {
		o := m[id]
		o.ID = id
		out[i] = o
	}
	return out
}

// pbGrade: % de rondes guanyades → -2..+2 (45-55 és igualat).
var pbGrade = [4]int{33, 45, 55, 64}

// Playbook: la taula matchupWin (la que calibra el motor), des del T i des del CT.
func (Game) Playbook() arena.Playbook {
	t, tv := make([][]int, len(stratVals)), make([][]int, len(stratVals))
	ct, ctv := make([][]int, len(setupVals)), make([][]int, len(setupVals))
	for i := range stratVals {
		for j := range setupVals {
			v := matchupWin[i][j]
			t[i], tv[i] = append(t[i], arena.Grade(v, pbGrade)), append(tv[i], v)
		}
	}
	for j := range setupVals {
		for i := range stratVals {
			v := 100 - matchupWin[i][j]
			ct[j], ctv[j] = append(ct[j], arena.Grade(v, pbGrade)), append(ctv[j], v)
		}
	}
	strats, setups := pbOpts(stratVals, pbStrats), pbOpts(setupVals, pbSetups)
	return arena.Playbook{Sections: []arena.PlaySection{
		{ID: "t", Title: "Attacking (T) vs their defense",
			Intro: "As T you choose how to attack before the round. The numbers are the % of rounds the T wins with equal full buys: find how they usually defend and pick the attack that beats it.",
			Mine:  strats, Theirs: setups, Grid: t, Values: tv, Unit: "% rounds won",
			Rule: "- As T, if they played {theirs} in most of their last 3 CT rounds: {mine}."},
		{ID: "ct", Title: "Defending (CT) vs their attack",
			Intro: "As CT you choose where to stand. Read where they attacked in past rounds and set up to stop it; a team that repeats itself gets punished.",
			Mine:  setups, Theirs: strats, Grid: ct, Values: ctv, Unit: "% rounds won",
			Rule: "- As CT, if they played {theirs} in most of their last 3 T rounds: {mine}."},
	}}
}
