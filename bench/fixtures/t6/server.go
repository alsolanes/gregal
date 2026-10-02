// Mini blog API: llista de posts en memòria amb botiga pròpia per mux.
package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Post és un apunt del blog.
type Post struct {
	ID    int    `json:"id"`
	Titol string `json:"titol"`
	Cos   string `json:"cos"`
}

type botiga struct {
	mu     sync.Mutex
	posts  []Post
	proper int
}

func novaBotiga() *botiga {
	return &botiga{posts: []Post{
		{ID: 1, Titol: "Benvinguts", Cos: "Primer post del blog."},
		{ID: 2, Titol: "Segon", Cos: "Un altre post."},
	}, proper: 3}
}

func escriuJSON(w http.ResponseWriter, codi int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(codi)
	json.NewEncoder(w).Encode(v)
}

func nouMux() *http.ServeMux {
	b := novaBotiga()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/posts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		escriuJSON(w, http.StatusOK, b.posts)
	})
	return mux
}

func main() {
	http.ListenAndServe("127.0.0.1:8472", nouMux())
}
