package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/harryyu02/chirpy/internal/database"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) getServerHits(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprintf(w, `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetServerHits(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return
	}
	apiCfg := &apiConfig{}
	dbQueries := database.New(db)
	apiCfg.db = dbQueries

	handler := http.NewServeMux()
	server := &http.Server{
		Handler: handler,
		Addr:    ":8080",
	}

	handler.Handle(
		"/app/",
		apiCfg.middlewareMetricsInc(
			http.StripPrefix("/app/", http.FileServer(http.Dir("."))),
		),
	)

	handler.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	handler.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		type chirp struct {
			Body string `json:"body"`
		}
		decoder := json.NewDecoder(r.Body)
		chirpBody := chirp{}
		err := decoder.Decode(&chirpBody)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(`{"error": "Something went wrong"}`))
			return
		}
		if len(chirpBody.Body) > 140 {
			w.WriteHeader(400)
			w.Write([]byte(`{"error": "Chirp is too long"}`))
			return
		}
		words := strings.Split(chirpBody.Body, " ")
		profaneMap := map[string]struct{}{
			"kerfuffle": {},
			"sharbert":  {},
			"fornax":    {},
		}
		for i, word := range words {
			if _, ok := profaneMap[strings.ToLower(word)]; ok {
				words[i] = "****"
			}
		}
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"cleaned_body": "%s"}`, strings.Join(words, " "))
	})

	handler.HandleFunc("GET /admin/metrics", apiCfg.getServerHits)
	handler.HandleFunc("POST /admin/reset", apiCfg.resetServerHits)

	server.ListenAndServe()
}
