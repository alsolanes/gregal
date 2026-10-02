package tools

import "testing"

// El journal és un de sol per procés i el web hi té diverses sessions
// alhora. Les marques de conversa anaven totes al mateix sac, així que
// rebobinar la sessió A podia agafar la marca que havia deixat la B i
// retallar-li la conversa a una llargada que no era seva: missatges
// perduts sense avís.
func TestLesMarquesDeConversaNoEsBarregenEntreSessions(t *testing.T) {
	j := NewJournal()
	dir := t.TempDir()
	a, b := dir+"/a.txt", dir+"/b.txt"

	j.SnapOp(a, "write") // seq 1
	j.MarkConvoFor("A", 10)
	j.SnapOp(b, "write") // seq 2
	j.MarkConvoFor("B", 2)

	if got := j.ConvoLenAtFor("A", 2); got != 10 {
		t.Errorf("la sessió A al seq 2 hauria de veure la seva marca (10), i veu %d", got)
	}
	if got := j.ConvoLenAtFor("B", 2); got != 2 {
		t.Errorf("la sessió B al seq 2 hauria de veure 2, i veu %d", got)
	}
	// Una sessió sense cap marca no ha d'heretar la d'una altra: -1 vol
	// dir "no en sé res", que és no retallar.
	if got := j.ConvoLenAtFor("C", 2); got != -1 {
		t.Errorf("la sessió C no ha marcat res: volem -1, tenim %d", got)
	}
	// El TUI i el Telegram marquen sense propietari i han de continuar
	// veient només les seves.
	j.MarkConvo(7)
	if got := j.ConvoLenAt(2); got != 7 {
		t.Errorf("sense propietari volem 7, tenim %d", got)
	}
	if got := j.ConvoLenAtFor("A", 2); got != 10 {
		t.Errorf("la marca sense propietari ha trepitjat la de A: %d", got)
	}
}
