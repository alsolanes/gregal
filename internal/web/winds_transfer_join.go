package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/skills"
)

// «Continua al servidor» dins d'un vent que ja hi és.
//
// La còpia de sempre crea un vent nou (copy-w-…). Si el servidor ja té un vent
// amb el mateix nom, acabaves amb dos «Garbí». Amb `into`, la conversa del
// mòbil s'afegeix al final de l'historial d'aquell vent, i el que s'hagi marcat
// (memòria, coneixements, skills) s'hi suma sense esborrar res del que ja sabia.
// Com la còpia, es pot repetir amb el mateix transferId sense duplicar res.

const transferJoinPhaseDone = "complete"

func (s *Server) joinTransfer(id, into string, b transferBundle) (transferResult, error) {
	if b.Kind != "wind" || len(b.Winds) != 1 || !transferValidID(into) {
		return transferResult{}, errTransferConflict
	}
	transferMu.Lock()
	defer transferMu.Unlock()
	windLibraryMu.Lock()
	defer windLibraryMu.Unlock()
	target, ok := s.winds.ByID(into)
	if !ok {
		return transferResult{}, errTransferConflict
	}
	digest := transferDigest(b) + ":into:" + into
	receiptPath := filepath.Join(s.windStateDir(), "wind-transfers", id+".json")
	result := transferResult{
		TransferID: id, Kind: "wind", ID: into,
		IDMap:      map[string]string{b.ID: into},
		VerifyPath: "/api/winds/chat?wind=" + into,
	}
	if raw, err := os.ReadFile(receiptPath); err == nil {
		var receipt transferReceipt
		if len(raw) > 64<<10 || json.Unmarshal(raw, &receipt) != nil {
			return transferResult{}, errTransferStorage
		}
		if receipt.Digest != digest {
			return transferResult{}, errTransferConflict
		}
		if receipt.Phase == transferJoinPhaseDone {
			receipt.Result.Replayed = true
			return receipt.Result, nil
		}
	} else if !os.IsNotExist(err) {
		return transferResult{}, errTransferStorage
	}
	if err := writeWindJSON(receiptPath, transferReceipt{Digest: digest, Phase: "staging", Result: result}); err != nil {
		return transferResult{}, errTransferStorage
	}

	incoming := make([]llm.Message, 0, len(b.History))
	for _, m := range b.History {
		incoming = append(incoming, llm.Message{Role: m.Role, Content: m.Text, Ts: m.Ts, Kind: m.Kind})
	}
	hist := windHistory(s, into)
	hist.mu.Lock()
	// Un reintent després d'una caiguda: si ja hi són (al final), no es tornen a posar.
	merged := hist.messages
	if !transferAlreadyAppended(merged, incoming) {
		merged = append(append([]llm.Message(nil), merged...), incoming...)
	}
	err := saveWindHistory(s, target, merged)
	if err == nil {
		hist.messages = merged
	}
	hist.mu.Unlock()
	if err != nil {
		return result, errTransferStorage
	}

	w := b.Winds[0]
	if strings.TrimSpace(w.Memory) != "" {
		cur := companion.LoadWindMemory(s.windStateDir(), into)
		if next := mergeTransferText(cur, w.Memory, companion.MaxWindMemoryBytes); next != cur {
			if companion.ReplaceWindMemory(s.windStateDir(), into, next) != nil {
				return result, errTransferStorage
			}
		}
	}
	if strings.TrimSpace(w.Knowledge) != "" {
		cur := s.windBrain(into)
		if next := mergeTransferText(cur, w.Knowledge, companion.MaxWindKnowledgeBytes); next != cur {
			if companion.ReplaceWindBrain(s.windStateDir(), into, next) != nil {
				return result, errTransferStorage
			}
		}
	}
	have := map[string]bool{}
	for _, sk := range s.windSkills(into) {
		have[strings.ToLower(sk.Nom)] = true
	}
	for _, sk := range w.Skills {
		if have[strings.ToLower(sk.Name)] {
			continue // la del servidor mana: no se sobreescriu res
		}
		if s.saveWindSkill(into, skills.Skill{Nom: sk.Name, Titol: sk.Title, Descripcio: sk.Description, Cos: sk.Body}) != nil {
			return result, errTransferStorage
		}
		result.Counts.Skills++
	}
	result.Counts.Winds = 1
	result.Counts.Messages = len(incoming)
	result.Counts.MemoryBytes = len(w.Memory)
	result.Counts.KnowledgeBytes = len(w.Knowledge)
	if err := writeWindJSON(receiptPath, transferReceipt{Digest: digest, Phase: transferJoinPhaseDone, Result: result}); err != nil {
		return result, errTransferStorage
	}
	return result, nil
}

// transferAlreadyAppended: l'historial ja acaba amb aquests missatges.
func transferAlreadyAppended(have, incoming []llm.Message) bool {
	if len(incoming) == 0 {
		return true
	}
	if len(have) < len(incoming) {
		return false
	}
	tail := have[len(have)-len(incoming):]
	for i := range incoming {
		if tail[i].Role != incoming[i].Role || tail[i].Content != incoming[i].Content || tail[i].Ts != incoming[i].Ts {
			return false
		}
	}
	return true
}

// mergeTransferText afegeix les línies noves al final, sense repetir-ne cap,
// fins al límit (el que no hi cap es queda al mòbil).
func mergeTransferText(cur, add string, max int) string {
	seen := map[string]bool{}
	for _, l := range strings.Split(cur, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			seen[t] = true
		}
	}
	out := strings.TrimRight(cur, "\n")
	for _, l := range strings.Split(add, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || seen[t] {
			continue
		}
		next := l
		if out != "" {
			next = out + "\n" + l
		}
		if len(next) > max {
			break
		}
		out = next
		seen[t] = true
	}
	if out == strings.TrimRight(cur, "\n") {
		return cur
	}
	return out + "\n"
}
