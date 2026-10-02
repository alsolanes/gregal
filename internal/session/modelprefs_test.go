package session

import "testing"

func TestModelDefaultsPersistAndRemainUserScoped(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	if err := SaveModelDefault("alice", "chat", "p1/model-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveModelDefault("alice", "code", "p2/model-b"); err != nil {
		t.Fatal(err)
	}
	if err := SaveModelDefault("alice smith", "chat", "other/model"); err != nil {
		t.Fatal(err)
	}
	if err := SaveModelDefault("alice-smith", "chat", "third/model"); err != nil {
		t.Fatal(err)
	}

	got, err := LoadModelDefaults("alice")
	if err != nil {
		t.Fatal(err)
	}
	if got["chat"] != "p1/model-a" || got["code"] != "p2/model-b" {
		t.Fatalf("alice defaults=%v", got)
	}
	spaced, err := LoadModelDefaults("alice smith")
	if err != nil {
		t.Fatal(err)
	}
	dashed, err := LoadModelDefaults("alice-smith")
	if err != nil {
		t.Fatal(err)
	}
	if spaced["chat"] != "other/model" || dashed["chat"] != "third/model" {
		t.Fatalf("distinct user defaults collided: spaced=%v dashed=%v", spaced, dashed)
	}
}
