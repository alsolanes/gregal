package lol

import "gregal/jocs/arena"

var pbStances = map[string]arena.PlayOption{
	"engage":  {Label: "Engage", Plain: "Force big 5v5 clashes. Needs tanks and engage champions; catches sieges and picks."},
	"siege":   {Label: "Siege", Plain: "Poke from range and take mid towers. Needs poke and waveclear; gets dived by engage and pick."},
	"pick":    {Label: "Pick", Plain: "Catch isolated enemies with assassins. Catches sieges and split pushers, gets caught by engage."},
	"split":   {Label: "Split push", Plain: "One strong duelist takes side towers while the team stalls. Beats engage and protect."},
	"protect": {Label: "Protect", Plain: "Peel for your carry and only clash together. Beats engage and pick, loses to siege and split."},
}

// Playbook: les postures de lluita (beats/versus, el que fa servir el motor).
func (Game) Playbook() arena.Playbook {
	opts := make([]arena.PlayOption, len(stances))
	grid := make([][]int, len(stances))
	for i, a := range stances {
		o := pbStances[a]
		o.ID = a
		opts[i] = o
		for _, b := range stances {
			grid[i] = append(grid[i], 2*int(versus(a, b)))
		}
	}
	return arena.Playbook{Sections: []arena.PlaySection{
		{ID: "stance", Title: "Your stance vs their stance",
			Intro: "Every 4 minutes both teams pick a stance for the next clashes. Each stance beats two and loses to two. A stance also works better when your champions fit it, so draft for the stances you want to play.",
			Mine:  opts, Theirs: opts, Grid: grid,
			Rule: "- If their most frequent stance so far is {theirs}: stance {mine}."},
	}}
}
