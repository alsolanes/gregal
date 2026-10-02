// Mini-app de prova: servidor HTTP amb un endpoint de salut.
package main

import (
	"encoding/json"
	"net/http"
)

func nouMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	return mux
}

func main() {
	http.ListenAndServe("127.0.0.1:8471", nouMux())
}
