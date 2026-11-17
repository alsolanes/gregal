package web

import (
	"strings"
	"testing"
)

func TestRotllanaWantsReview(t *testing.T) {
	long := strings.Repeat("Una resposta ben llarga i concreta. ", 8)
	for name, tc := range map[string]struct {
		directed, cont bool
		intent, reply  string
		want           bool
	}{
		"normal":        {false, false, "", long, true},
		"adreçat":       {true, false, "", long, false},
		"continua":      {false, true, "", long, false},
		"ja és revisió": {false, false, "review", long, false},
		"curta":         {false, false, "", "Fet.", false},
		"pregunta":      {false, false, "", long + " Quin prefereixes?", true},
		"opcions":       {false, false, "", long + "\n```choices\nA\nB\n```", true},
	} {
		if got := rotllanaWantsReview(tc.directed, tc.cont, tc.intent, tc.reply); got != tc.want {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
}

func TestRotllanaReviewerIsAnotherMember(t *testing.T) {
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, _ := s.rotllanaByID(id)
	members := s.rotMembersOf(rot)
	rev, ok := s.rotllanaReviewer(members, members[:1], "revisa això", nil)
	if !ok || rev.ID == members[0].ID {
		t.Fatalf("revisor %+v ok=%v", rev, ok)
	}
	if _, ok := s.rotllanaReviewer(members, members, "x", nil); ok {
		t.Fatal("si tots han parlat no hi ha revisor")
	}
}
