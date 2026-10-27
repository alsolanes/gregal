package web

import "encoding/json"

// histMsg llegeix un missatge de l'historial com a text (from, text,
// speaker_id…) i ignora els camps que no ho són, com l'hora (ts).
type histMsg map[string]string

func (h *histMsg) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*h = histMsg{}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			(*h)[k] = s
		}
	}
	return nil
}
