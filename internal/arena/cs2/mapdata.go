package cs2

import "math"

// Mirage simplificat: un graf de zones amb coordenades 2D (0..100 × 0..80,
// y cap avall). T a la dreta, CT a l'esquerra, A a baix, B a dalt, com al radar.

type zone struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Kind string  `json:"kind"` // spawn_t, spawn_ct, site, area
	Site string  `json:"site,omitempty"`
}

type prop struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

const (
	mapW = 100.0
	mapH = 80.0
)

var zones = []zone{
	{"t_spawn", "T Spawn", 90, 42, 14, 22, "spawn_t", ""},
	{"a_main", "T Ramp / A Main", 70, 64, 16, 9, "area", "A"},
	{"palace", "Palace", 62, 75, 12, 7, "area", "A"},
	{"a_site", "A Site", 40, 65, 20, 15, "site", "A"},
	{"ct", "CT", 22, 70, 10, 9, "area", "A"},
	{"ct_spawn", "CT Spawn", 11, 50, 12, 14, "spawn_ct", ""},
	{"jungle", "Jungle / Connector", 36, 50, 10, 9, "area", "A"},
	{"mid", "Mid", 56, 42, 16, 9, "area", ""},
	{"top_mid", "Top Mid", 74, 40, 9, 12, "area", ""},
	{"window", "Window", 27, 39, 8, 7, "area", ""},
	{"short", "Short (Catwalk)", 41, 29, 14, 6, "area", "B"},
	{"b_apps", "B Apartments", 56, 13, 18, 8, "area", "B"},
	{"b_site", "B Site", 24, 15, 18, 13, "site", "B"},
	{"market", "Market", 13, 31, 10, 9, "area", "B"},
}

var links = [][2]string{
	{"t_spawn", "a_main"}, {"t_spawn", "palace"}, {"t_spawn", "top_mid"}, {"t_spawn", "b_apps"},
	{"a_main", "a_site"}, {"palace", "a_site"}, {"a_site", "ct"}, {"a_site", "jungle"},
	{"ct", "ct_spawn"}, {"jungle", "mid"}, {"jungle", "window"}, {"mid", "top_mid"},
	{"mid", "short"}, {"window", "short"}, {"short", "b_site"}, {"b_apps", "b_site"},
	{"b_site", "market"}, {"market", "ct_spawn"}, {"jungle", "ct_spawn"},
}

var props = []prop{
	{36, 62, 3, 3}, {44, 69, 4, 2.5}, {47, 60, 2.5, 3}, {33, 70, 3, 2}, // A: default, triple, firebox, tetris
	{21, 12, 6, 2.5}, {28, 18, 2.5, 3}, {18, 19, 2.5, 2.5}, // B: van, bench, boxes
	{57, 41, 3, 2}, {70, 66, 2, 3}, {56, 15, 2.5, 2},
}

// On es planta a cada lloc.
var plantSpot = map[string][2]float64{"A": {41, 66}, "B": {24, 15}}

var zoneByID = func() map[string]*zone {
	m := map[string]*zone{}
	for i := range zones {
		m[zones[i].ID] = &zones[i]
	}
	return m
}()

func dist(ax, ay, bx, by float64) float64 { return math.Hypot(ax-bx, ay-by) }

// paths: el camí més curt entre zones (centres), precalculat.
var pathTable = func() map[[2]string][]string {
	adj := map[string][]string{}
	for _, l := range links {
		adj[l[0]] = append(adj[l[0]], l[1])
		adj[l[1]] = append(adj[l[1]], l[0])
	}
	out := map[[2]string][]string{}
	for _, s := range zones {
		d := map[string]float64{s.ID: 0}
		prev := map[string]string{}
		done := map[string]bool{}
		for {
			best, bd := "", math.Inf(1)
			for _, z := range zones {
				if v, ok := d[z.ID]; ok && !done[z.ID] && v < bd {
					best, bd = z.ID, v
				}
			}
			if best == "" {
				break
			}
			done[best] = true
			a := zoneByID[best]
			for _, n := range adj[best] {
				b := zoneByID[n]
				nd := bd + dist(a.X, a.Y, b.X, b.Y)
				if v, ok := d[n]; !ok || nd < v {
					d[n] = nd
					prev[n] = best
				}
			}
		}
		for _, z := range zones {
			var p []string
			for c := z.ID; ; c = prev[c] {
				p = append([]string{c}, p...)
				if c == s.ID {
					break
				}
				if _, ok := prev[c]; !ok {
					p = nil
					break
				}
			}
			out[[2]string{s.ID, z.ID}] = p
		}
	}
	return out
}()

func path(from, to string) []string {
	if from == to {
		return []string{to}
	}
	return pathTable[[2]string{from, to}]
}

// siteZones: les posicions que compten com a defensa de cada lloc.
func siteOf(z string) string {
	if zz := zoneByID[z]; zz != nil {
		return zz.Site
	}
	return ""
}
