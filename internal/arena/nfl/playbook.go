package nfl

import (
	"gregal/internal/arena"
	"gregal/internal/football"
)

// Les jugades amb contrajoc (les de la taula Matchup), en l'ordre de la taula.
var pbPlays = []arena.PlayOption{
	{ID: "run_inside", Label: "Inside run", Plain: "Hand it off up the middle. Steady yards when the defense drops back to cover passes."},
	{ID: "run_outside", Label: "Outside run", Plain: "Run to the edge. Great against man coverage, stopped by a cover3 crowd."},
	{ID: "qb_sneak", Label: "QB sneak", Plain: "The quarterback pushes forward for 1-2 yards. Only for 3rd or 4th & 1."},
	{ID: "pass_short", Label: "Short pass", Plain: "Quick 3-9 yard throw. Safe, and it punishes blitzes and deep zones."},
	{ID: "pass_medium", Label: "Medium pass", Plain: "10-18 yard throw. Big gains against soft zones, sacked by a blitz."},
	{ID: "pass_deep", Label: "Deep pass", Plain: "20+ yard shot. Huge against man coverage, intercepted against cover2."},
	{ID: "screen", Label: "Screen", Plain: "Let the rush through and dump it to the running back. The blitz killer."},
	{ID: "play_action", Label: "Play-action", Plain: "Fake the run, then throw. Works once you have run the ball (40%+ of plays)."},
}

var pbDefs = []arena.PlayOption{
	{ID: "base", Label: "Base", Plain: "Balanced 4-3. Never terrible, rarely great."},
	{ID: "blitz", Label: "Blitz", Plain: "Send extra rushers at the quarterback. Stops runs and medium passes, burned by screens."},
	{ID: "cover2", Label: "Cover 2", Plain: "Two deep safeties: no deep passes, but the middle is open for runs."},
	{ID: "cover3", Label: "Cover 3", Plain: "Eight in the box and three deep: shuts down runs to the edge."},
	{ID: "man", Label: "Man", Plain: "Every receiver has a shadow: short passes and screens die, deep shots win."},
	{ID: "goal_line", Label: "Goal line", Plain: "Everyone at the line. For short yardage only: passes beat it easily."},
	{ID: "prevent", Label: "Prevent", Plain: "Soft deep zone to stop the long bomb at the end of a half. Runs walk through it."},
}

// Playbook: la taula football.Matchup vista des de l'atac i des de la defensa.
func (Game) Playbook() arena.Playbook {
	off := make([][]int, len(pbPlays))
	def := make([][]int, len(pbDefs))
	for i, p := range pbPlays {
		for _, d := range pbDefs {
			off[i] = append(off[i], football.Matchup[p.ID][d.ID])
		}
	}
	for j, d := range pbDefs {
		for _, p := range pbPlays {
			def[j] = append(def[j], -football.Matchup[p.ID][d.ID])
		}
	}
	return arena.Playbook{Sections: []arena.PlaySection{
		{ID: "offense", Title: "Your offense vs their defense",
			Intro: "Every snap both teams call at the same time. Each play beats some defenses and loses to others, like rock-paper-scissors: find the defense they call most and pick a play that beats it.",
			Mine:  pbPlays, Theirs: pbDefs, Grid: off,
			Rule: "- If their most-called defense is {theirs}: call {mine}."},
		{ID: "defense", Title: "Your defense vs their offense",
			Intro: "On defense you guess what they will run. Look at the play they call most and choose the coverage that stops it.",
			Mine:  pbDefs, Theirs: pbPlays, Grid: def,
			Rule: "- If their most-called play is {theirs}: call {mine}."},
	}}
}
