package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/harryyu02/chirpy/internal/auth"
	"github.com/harryyu02/chirpy/internal/database"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

type chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

type user struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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
	if cfg.platform != "dev" {
		w.WriteHeader(403)
	}
	cfg.fileserverHits.Store(0)
	err := cfg.db.DeleteAllUsers(context.Background())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong"}`))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	type createUser struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	createUserArgs := createUser{}
	err := decoder.Decode(&createUserArgs)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - parse args failed"}`))
		return
	}
	hashedPassword, err := auth.HashPassword(createUserArgs.Password)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - hashing password failed"}`))
		fmt.Printf("err.Error(): %v\n", err.Error())
		return
	}
	created, err := cfg.db.CreateUser(context.Background(), database.CreateUserParams{
		Email: createUserArgs.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - create user failed"}`))
		fmt.Printf("err.Error(): %v\n", err.Error())
		return
	}
	createdUser := user{
		ID:        created.ID,
		CreatedAt: created.CreatedAt,
		UpdatedAt: created.UpdatedAt,
		Email:     created.Email,
	}
	createdBytes, err := json.Marshal(createdUser)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - user invalid"}`))
		return
	}

	w.WriteHeader(201)
	w.Write(createdBytes)
}

func (cfg *apiConfig) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	type createChirp struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}
	decoder := json.NewDecoder(r.Body)
	createChirpArgs := createChirp{}
	err := decoder.Decode(&createChirpArgs)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - parse args failed"}`))
		return
	}
	userId, err := uuid.Parse(createChirpArgs.UserId)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - parse user_id failed"}`))
		fmt.Printf("err.Error(): %v\n", err.Error())
		return
	}

	if len(createChirpArgs.Body) > 140 {
		w.WriteHeader(400)
		w.Write([]byte(`{"error": "Chirp is too long"}`))
		return
	}
	words := strings.Split(createChirpArgs.Body, " ")
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
	cleanedBody := strings.Join(words, " ")

	created, err := cfg.db.CreateChirp(context.Background(), database.CreateChirpParams{
		Body:   cleanedBody,
		UserID: userId,
	})

	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - create chirp failed"}`))
		fmt.Printf("err.Error(): %v\n", err.Error())
		return
	}
	createdChirp := chirp{
		ID:        created.ID,
		CreatedAt: created.CreatedAt,
		UpdatedAt: created.UpdatedAt,
		Body:      created.Body,
		UserID:    created.UserID,
	}
	createdBytes, err := json.Marshal(createdChirp)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - chirp invalid"}`))
		return
	}

	w.WriteHeader(201)
	w.Write(createdBytes)
}

func (cfg *apiConfig) handleGetChirps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	chirps, err := cfg.db.GetChirps(context.Background())
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - get chirps failed"}`))
		fmt.Printf("err.Error(): %v\n", err.Error())
		return
	}
	chirpsJson := make([]chirp, len(chirps))
	for i, c := range chirps {
		chirpsJson[i] = chirp{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		}
	}

	gotBytes, err := json.Marshal(chirpsJson)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - chirp invalid"}`))
		return
	}

	w.WriteHeader(200)
	w.Write(gotBytes)
}

func (cfg *apiConfig) handleGetChirpByID(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	chirpUUID, err := uuid.Parse(chirpID)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - chirp id invalid"}`))
		return
	}
	c, err := cfg.db.GetChirpByID(context.Background(), chirpUUID)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(`{"error": "Something went wrong - chirp id not found"}`))
		return
	}

	chirpsJson := chirp{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Body:      c.Body,
		UserID:    c.UserID,
	}
	gotBytes, err := json.Marshal(chirpsJson)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - chirp invalid"}`))
		return
	}
	w.WriteHeader(200)
	w.Write(gotBytes)
}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type login struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	w.Header().Set("Content-Type", "application/json")
	decoder := json.NewDecoder(r.Body)
	loginArgs := login{}
	err := decoder.Decode(&loginArgs)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - parse args failed"}`))
		return
	}
	u, err := cfg.db.GetUserByEmail(context.Background(), loginArgs.Email)
	if err != nil {
		w.WriteHeader(401)
		w.Write([]byte(`{"error": "Something went wrong - user not found"}`))
		return
	}
	match, err := auth.CheckPasswordHash(loginArgs.Password, u.HashedPassword)
	if err != nil || !match {
		w.WriteHeader(401)
		w.Write([]byte(`{"error": "Something went wrong - password is incorrect"}`))
		return
	}

	loggedInUser := user{
		ID:        u.ID,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		Email:     u.Email,
	}
	loggedInBytes, err := json.Marshal(loggedInUser)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(`{"error": "Something went wrong - user invalid"}`))
		return
	}
	w.WriteHeader(200)
	w.Write(loggedInBytes)
}

func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return
	}
	apiCfg := &apiConfig{}
	dbQueries := database.New(db)
	apiCfg.db = dbQueries
	apiCfg.platform = platform

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
	handler.HandleFunc("POST /api/users", apiCfg.handleCreateUser)
	handler.HandleFunc("POST /api/login", apiCfg.handleLogin)
	handler.HandleFunc("GET /api/chirps", apiCfg.handleGetChirps)
	handler.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handleGetChirpByID)
	handler.HandleFunc("POST /api/chirps", apiCfg.handleCreateChirp)

	handler.HandleFunc("GET /admin/metrics", apiCfg.getServerHits)
	handler.HandleFunc("POST /admin/reset", apiCfg.resetServerHits)

	server.ListenAndServe()
}
