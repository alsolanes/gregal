// Package events desa una seqüència append-only d'esdeveniments del servei.
//
// El fitxer és JSONL perquè una interrupció només pugui perdre l'última línia;
// els clients llegeixen amb un cursor monotònic i poden reprendre una
// connexió SSE sense tornar a executar el torn.
package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Event struct {
	ID        uint64    `json:"id"`
	At        time.Time `json:"at"`
	SessionID string    `json:"session_id,omitempty"`
	User      string    `json:"user,omitempty"`
	RunID     int       `json:"run_id,omitempty"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	// Payload és opcional per mantenir compatibles els events antics, que
	// només tenien text. Quan existeix, conté JSON estructurat del contracte
	// interactiu (p. ex. approve_request o question_request).
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	next    uint64
	offsets map[uint64]int64 // posició just després de cada event
}

// Open obre (o crea) un magatzem i recupera l'últim cursor escrit.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("events: camí buit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	s := &Store{path: path, offsets: make(map[uint64]int64)}
	if err := s.scan(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) scan() error {
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	lines := bytes.Split(raw, []byte{'\n'})
	offset := int64(0)
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			offset += int64(len(line) + 1)
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			// Un procés mort pot deixar només l'última línia a mig escriure.
			// Les línies anteriors són vàlides i el cursor es pot recuperar.
			if i == len(lines)-1 || (i == len(lines)-2 && len(lines[len(lines)-1]) == 0) {
				cut := len(bytes.Join(lines[:i], []byte{'\n'}))
				if i > 0 {
					cut++
				} // el salt de línia que precedeix el tros trencat
				_ = os.Truncate(s.path, int64(cut))
				break
			}
			return fmt.Errorf("events: línia il·legible: %w", err)
		}
		if e.ID > s.next {
			s.next = e.ID
		}
		s.offsets[e.ID] = offset + int64(len(line)+1)
		offset += int64(len(line) + 1)
	}
	return nil
}

func (s *Store) Append(e Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	e.ID = s.next
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	} else {
		e.At = e.At.UTC()
	}
	b, err := json.Marshal(e)
	if err != nil {
		s.next--
		return Event{}, err
	}
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		s.next--
		return Event{}, err
	}
	start, seekErr := f.Seek(0, io.SeekEnd)
	if seekErr != nil {
		_ = f.Close()
		s.next--
		return Event{}, seekErr
	}
	_, err = f.Write(append(b, '\n'))
	closeErr := f.Close()
	if err != nil {
		s.next--
		return Event{}, err
	}
	if closeErr != nil {
		return Event{}, closeErr
	}
	s.offsets[e.ID] = start + int64(len(b)+1)
	return e, nil
}

// Cursor és l'últim identificador escrit. Un client pot començar a
// escoltar des d'aquí sense reproduir totes les tasques antigues.
func (s *Store) Cursor() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.next
}

// After torna events amb id estrictament posterior al cursor.
func (s *Store) After(after uint64, limit int) ([]Event, error) {
	return s.after(after, limit, nil)
}

// AfterUser és la variant filtrada per usuari. El servei comparteix un fitxer
// append-only entre usuaris, però mai no retorna els events d'un altre.
func (s *Store) AfterUser(after uint64, limit int, user string) ([]Event, error) {
	return s.after(after, limit, func(e Event) bool { return e.User == user })
}

func (s *Store) AfterScope(after uint64, limit int, user, sessionID string) ([]Event, error) {
	return s.after(after, limit, func(e Event) bool {
		return e.User == user && (sessionID == "" || e.SessionID == sessionID)
	})
}

func (s *Store) after(after uint64, limit int, keep func(Event) bool) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if after >= s.next {
		return nil, nil
	}
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if offset, ok := s.offsets[after]; ok {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
	}
	out := make([]Event, 0, limit)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("events: línia il·legible: %w", err)
		}
		if e.ID <= after {
			continue
		}
		if keep != nil && !keep(e) {
			continue
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) Close() error {
	return nil
}
