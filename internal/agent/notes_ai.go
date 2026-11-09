package agent

import (
	"context"
	"errors"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Accions d'IA sobre una nota de l'usuari. Cap no escriu res: tornen el
// markdown proposat i és la finestra qui el mostra perquè l'usuari
// l'accepti o el descarti.
var noteAIActions = map[string]string{
	"tidy": "Clean up this note: fix spelling, punctuation and obvious typos, " +
		"turn loose lines into proper markdown (headings, lists, `- [ ]` tasks, code blocks, tables) " +
		"and remove duplicated lines. Keep every idea, fact, link and task; do not add new content.",
	"organize": "Reorganize this note: group related content under short `##` headings in a logical order, " +
		"collect action items into a `- [ ]` task list, and keep everything else. " +
		"Do not invent content and do not drop any fact, link or task.",
	"summarize": "Add a short `> ` summary (one to three sentences) at the very top of this note, " +
		"then keep the original note unchanged below it.",
	"tasks": "Extract every action item from this note into a `## Tasks` section of `- [ ]` checkboxes " +
		"at the end (keep done items as `- [x]`). Leave the rest of the note as it is.",
}

const noteAISystem = "You edit a user's personal note written in markdown. " +
	"Always answer in the same language as the note. " +
	"Reply with ONLY the full resulting note in markdown: no preamble, no explanation, no code fence around it. " +
	"Never include the note title as a heading unless it was already there."

// NoteAIAction diu si l'acció existeix ("custom" necessita instrucció).
func NoteAIAction(action string) bool {
	_, ok := noteAIActions[action]
	return ok || action == "custom"
}

// NoteAI proposa el cos nou d'una nota segons l'acció. instruction és el que
// demana l'usuari amb «custom» (i s'afegeix a les altres si n'hi ha).
func NoteAI(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, r config.Role, action, instruction, title, body string) (string, error) {
	if strings.TrimSpace(body) == "" && action != "custom" {
		return "", errors.New("the note is empty")
	}
	task := noteAIActions[action]
	instruction = strings.TrimSpace(instruction)
	if action == "custom" {
		if instruction == "" {
			return "", errors.New("say what to do with the note")
		}
		task = "Apply this request to the note: " + instruction
	} else if task == "" {
		return "", errors.New("unknown action")
	} else if instruction != "" {
		task += "\nAlso: " + instruction
	}
	user := task + "\n\nTitle: " + title + "\n\n---\n" + body
	out, err := sideCallLateral(ctx, client, cfg, p, r, []llm.Message{
		{Role: "system", Content: noteAISystem},
		{Role: "user", Content: user},
	})
	if err != nil {
		return "", err
	}
	out = CleanNoteAI(out)
	if out == "" {
		return "", errors.New("the model returned an empty note")
	}
	return out, nil
}

// CleanNoteAI treu el que els models hi afegeixen tot i demanar-los que no:
// una tanca ``` al voltant de tota la nota o una línia «---» inicial.
func CleanNoteAI(out string) string {
	out = strings.TrimSpace(out)
	if strings.HasPrefix(out, "```") && strings.HasSuffix(out, "```") && len(out) > 6 {
		inner := strings.TrimSuffix(out, "```")
		if i := strings.IndexByte(inner, '\n'); i >= 0 {
			out = strings.TrimSpace(inner[i+1:])
		}
	}
	out = strings.TrimSpace(strings.TrimPrefix(out, "---\n"))
	return out
}
