package web

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// windNamedIn returns a uniquely named member, at a word boundary.
func windNamedIn(members []companion.Wind, msg string) (companion.Wind, bool) {
	found := windsNamedIn(members, msg)
	if len(found) == 1 {
		return found[0], true
	}
	return companion.Wind{}, false
}

func windsNamedIn(members []companion.Wind, msg string) []companion.Wind {
	low := strings.ToLower(msg)
	names := map[string]int{}
	for _, w := range members {
		names[strings.ToLower(strings.TrimSpace(w.Name))]++
	}
	var found []companion.Wind
	pos := map[string]int{}
	for _, w := range members {
		name := strings.ToLower(strings.TrimSpace(w.Name))
		if name == "" || names[name] != 1 {
			continue
		}
		for i := strings.Index(low, name); i >= 0; {
			end := i + len(name)
			before, after := ' ', ' '
			if i > 0 {
				before = []rune(low[:i])[len([]rune(low[:i]))-1]
			}
			if end < len(low) {
				after = []rune(low[end:])[0]
			}
			if !unicode.IsLetter(before) && !unicode.IsLetter(after) {
				found = append(found, w)
				pos[w.ID] = i
				break
			}
			next := strings.Index(low[end:], name)
			if next < 0 {
				break
			}
			i = end + next
		}
	}
	// En l'ordre en què l'usuari els nomena: «@Llevant i @Xaloc» → primer el Llevant.
	sort.SliceStable(found, func(a, b int) bool { return pos[found[a].ID] < pos[found[b].ID] })
	return found
}

// routeRotllanaSpeakers is deliberately local: choosing a speaker must not
// consume a model slot or wait for a separate inference before the actual reply.
// An explicit member wins; otherwise preserve the classic participation rule.
func (s *Server) routeRotllanaSpeakers(ctx context.Context, rot Rotllana, wanted, msg string, tail []llm.Message) []companion.Wind {
	if ctx.Err() != nil {
		return nil
	}
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		return nil
	}
	if wanted != "" {
		for _, w := range members {
			if w.ID == wanted {
				return []companion.Wind{w}
			}
		}
		return nil
	}
	if named := windsNamedIn(members, msg); len(named) > 0 {
		if len(named) > 2 {
			named = named[:2]
		}
		return named
	}
	return classicRotllanaSpeaker(members, tail)
}

// rotllanaTurnSpeakers keeps continuation on the last actual voice, rather
// than asking the next least-represented wind to finish somebody else's reply.
func (s *Server) rotllanaTurnSpeakers(ctx context.Context, rot Rotllana, wanted, msg string, tail []llm.Message, continuing bool) []companion.Wind {
	if continuing && wanted == "" {
		members := s.rotMembersOf(rot)
		for i := len(tail) - 1; i >= 0; i-- {
			m := tail[i]
			if m.Role != "assistant" {
				continue
			}
			for _, w := range members {
				if m.SpeakerID == w.ID {
					wanted = w.ID
					break
				}
			}
			if wanted == "" && m.SpeakerID == "" {
				if prefix, _, ok := strings.Cut(m.Content, ":"); ok {
					if w, ok := memberByPrefix(members, prefix); ok {
						wanted = w.ID
					}
				}
			}
			if wanted != "" {
				break
			}
		}
	}
	return s.routeRotllanaSpeakers(ctx, rot, wanted, msg, tail)
}

// classicRotllanaSpeaker balances participation in the last six messages.
// Stable roster order breaks ties. Legacy history without speaker IDs works too.
func classicRotllanaSpeaker(members []companion.Wind, tail []llm.Message) []companion.Wind {
	if len(members) == 0 {
		return nil
	}
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	counts := map[string]int{}
	for _, m := range tail {
		if m.Role != "assistant" {
			continue
		}
		for _, w := range members {
			if m.SpeakerID == w.ID || (m.SpeakerID == "" && strings.HasPrefix(m.Content, w.Name+":")) {
				counts[w.ID]++
			}
		}
	}
	best := members[0]
	for _, w := range members[1:] {
		if counts[w.ID] < counts[best.ID] {
			best = w
		}
	}
	return []companion.Wind{best}
}
