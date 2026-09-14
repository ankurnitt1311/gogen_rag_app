package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type user struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Plan  string `json:"plan"`
}

func main() {
	addr := os.Getenv("USER_SVC_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	users := map[string]user{
		"1": {ID: "1", Name: "Ada Lovelace", Email: "ada@acme.com", Plan: "enterprise"},
		"2": {ID: "2", Name: "Grace Hopper", Email: "grace@acme.com", Plan: "standard"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
		if email == "" {
			http.Error(w, `{"error":"email query is required"}`, http.StatusBadRequest)
			return
		}
		for _, u := range users {
			if strings.ToLower(u.Email) == email {
				writeUser(w, u)
				return
			}
		}
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := users[r.PathValue("id")]
		if !ok {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		writeUser(w, u)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if !strings.HasPrefix(addr, ":") && !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		panic(err)
	}
}

func writeUser(w http.ResponseWriter, u user) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(u)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
