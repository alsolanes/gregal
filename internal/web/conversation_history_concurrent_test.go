package web

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/session"
)

func TestConversationHistorySerializesWholeTurn(t *testing.T) {
	for _, route := range historyRoutes {
		t.Run(route.kind, func(t *testing.T) {
			a, b, _ := historyFixture(t, route, false)
			assertHistory(t, route.get(a), 0)
			assertHistory(t, route.get(b), 0)
			firstEntered, secondEntered, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			historySeam(t, func(prompt string) (string, error) {
				n := calls.Add(1)
				if n == 1 {
					close(firstEntered)
					<-release
				} else {
					close(secondEntered)
					if !strings.Contains(prompt, "PRIVATE-ANSWER-1") {
						t.Error("second model ran without the first committed reply")
					}
				}
				return fmt.Sprintf("PRIVATE-ANSWER-%d", n), nil
			})
			firstDone, secondDone, getDone := make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1)
			go func() { firstDone <- route.post(a, "PRIVATE-QUESTION-1") }()
			<-firstEntered
			go func() { secondDone <- route.post(b, "PRIVATE-QUESTION-2") }()
			go func() { getDone <- route.get(b) }()
			select {
			case <-secondEntered:
				t.Error("second model overtook the pending first turn")
			case <-time.After(100 * time.Millisecond):
			}
			gotEarly := false
			select {
			case out := <-getDone:
				gotEarly = true
				assertHistory(t, out, 0)
			case <-time.After(20 * time.Millisecond):
			}
			close(release)
			assertHistory(t, <-firstDone, 1)
			assertHistory(t, <-secondDone, 2)
			if !gotEarly {
				out := <-getDone
				if out.Code != 200 {
					t.Fatal(out.Body.String())
				}
			}
			assertHistory(t, route.get(a), 2)
			saved, err := session.Load(a.sessDir(), route.durable)
			if err != nil {
				t.Fatal(err)
			}
			if len(saved.Convo) != 4 || saved.Convo[0].Content != "PRIVATE-QUESTION-1" || !strings.Contains(saved.Convo[1].Content, "PRIVATE-ANSWER-1") || saved.Convo[2].Content != "PRIVATE-QUESTION-2" || !strings.Contains(saved.Convo[3].Content, "PRIVATE-ANSWER-2") {
				t.Fatalf("non-transactional durable turn order: %+v", saved.Convo)
			}
		})
	}
}
