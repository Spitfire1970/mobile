package main

import _ "github.com/lib/pq"
import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"mobile-haha/internal/database"
	"mobile-haha/internal/auth"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type LoginUser struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token string `json:"token"`
	Refresh string `json:"refresh_token"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
}

type Token struct {
	Token string `json:"token"`
}

type failVals struct {
	Err string `json:"error"`
}

type apiConfig struct {
	fileserverHits atomic.Int32
	query          *database.Queries
	env            string
	secret string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(rw, req)
	})
}
func health(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	rw.Write([]byte("OK"))
}

func (cfg *apiConfig) metrics(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/html")
	s := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())
	fmt.Fprintf(rw, "%s", s)
}

func (cfg *apiConfig) reset(rw http.ResponseWriter, req *http.Request) {
	if cfg.env != "dev" {
		rw.WriteHeader(403)
		return
	}
	cfg.fileserverHits.Store(0)
	cfg.query.DeleteAllUsers(req.Context())
	rw.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) chirps(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "application/json")

	chirps, err := cfg.query.GetChirps(req.Context())
	if err != nil {
		rw.WriteHeader(500)
		fmt.Printf("%s", err.Error())
		return
	}
	n := make([]Chirp, 0)
	for _, chirp := range chirps {
		n = append(n, Chirp{
		ID:        chirp.ID,
		Body:      chirp.Body,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserId:    chirp.UserID,
	})
	}
	dat, _ := json.Marshal(n)
	rw.WriteHeader(200)
	rw.Write(dat)
}

func (cfg *apiConfig) getChirp(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "application/json")

	i := req.PathValue("chirpID")
	fmt.Printf("%s", i)
	id, err := uuid.Parse(i)
	if err != nil {
		rw.WriteHeader(500)
		fmt.Printf("%s", err.Error())
		return
	}

	chirp, err := cfg.query.GetChirp(req.Context(), id)
	fmt.Printf("%s", i)
	if err != nil {
		if err == sql.ErrNoRows {
            rw.WriteHeader(404)
            return
        }
		rw.WriteHeader(500)
		fmt.Printf("%s", err.Error())
		return
	}
	dat, _ := json.Marshal(Chirp{
		ID:        chirp.ID,
		Body:      chirp.Body,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserId:    chirp.UserID,
	})
	rw.WriteHeader(200)
	rw.Write(dat)
}

func (cfg *apiConfig) chirp(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	type parameters struct {
		Chirp  string    `json:"body"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	jwt, err1 := auth.GetBearerToken(req.Header)
	if err1 != nil {
		rw.WriteHeader(401)
		return
	}
	u, err2 := auth.ValidateJWT(jwt,cfg.secret)
	if err2 != nil {
		rw.WriteHeader(401)
		return
	}
	err := decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		res := failVals{
			Err: "Something went wrong",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	if len(params.Chirp) > 150 {
		rw.WriteHeader(400)
		res := failVals{
			Err: "Chirp is too long",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	s := params.Chirp
	split := strings.Split(s, " ")
	n := make([]string, 0)
	for _, word := range split {
		l := strings.ToLower(word)
		var new string
		if l != "kerfuffle" && l != "sharbert" && l != "fornax" {
			new = word
		} else {
			new = "****"
		}
		n = append(n, new)
	}
	chirp, err := cfg.query.CreateChirp(req.Context(), database.CreateChirpParams{Body: strings.Join(n, " "), UserID: u})
	if err != nil {
		rw.WriteHeader(500)
	}
	respBody := Chirp{
		ID:        chirp.ID,
		Body:      chirp.Body,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserId:    chirp.UserID,
	}
	dat, _ := json.Marshal(respBody)
	rw.WriteHeader(201)
	rw.Write(dat)
}

func (cfg *apiConfig) addUser(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	type parameters struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		res := failVals{
			Err: "Something went wrong",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	p, _ := auth.HashPassword(params.Password)
	user, err := cfg.query.CreateUser(req.Context(), database.CreateUserParams{Email: params.Email, HashedPassword: p})
	if err != nil {
		rw.WriteHeader(500)
		fmt.Printf("%s", err.Error())
	}
	respBody := User{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	dat, _ := json.Marshal(respBody)
	rw.WriteHeader(201)
	rw.Write(dat)
}


func (cfg *apiConfig) refresh(rw http.ResponseWriter, req *http.Request) {
	b, err := auth.GetBearerToken(req.Header)
	if b == "" || err != nil {
		rw.WriteHeader(401)
		return
	}
	r, err := cfg.query.GetRefToken(req.Context(), b)
	if err != nil {
		rw.WriteHeader(401)
		return	
	}
	expired := r.ExpiresAt.Before(time.Now())
	if expired || r.RevokedAt.Valid {
		rw.WriteHeader(401)
		return	
	}
	exp := time.Duration(3600) * time.Second
	user, err := cfg.query.GetUserFromRefreshToken(req.Context(), r.Token)
	if err != nil {
		rw.WriteHeader(401)
		return	
	}
	t, _ := auth.MakeJWT(user.ID, cfg.secret, exp)
	respBody := Token{
		Token: t,
	}
	dat, _ := json.Marshal(respBody)
	rw.WriteHeader(200)
	rw.Write(dat)
}
func (cfg *apiConfig) revoke(rw http.ResponseWriter, req *http.Request) {
	b, err := auth.GetBearerToken(req.Header)
	if b == "" || err != nil {
		rw.WriteHeader(401)
		return
	}
	r, err := cfg.query.GetRefToken(req.Context(), b)
	if err != nil {
		rw.WriteHeader(401)
		return	
	}
	expired := r.ExpiresAt.Before(time.Now())
	if expired {
		rw.WriteHeader(401)
		return	
	}
	cfg.query.UpdateRefToken(req.Context(), r.Token)
	rw.WriteHeader(204)
}

func (cfg *apiConfig) login(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
	type parameters struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		res := failVals{
			Err: "Something went wrong",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	user, err := cfg.query.GetUserByEmail(req.Context(), params.Email)
	if err != nil {
		rw.WriteHeader(401)
		res := failVals{
			Err: "Incorrect email or password",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	b, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil || !b {
		rw.WriteHeader(401)
		res := failVals{
			Err: "Incorrect email or password",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	exp := time.Duration(3600) * time.Second
	raw, _ := auth.MakeRefreshToken()
	r, _ := cfg.query.CreateRefToken(req.Context(), database.CreateRefTokenParams{UserID: user.ID, Token: raw, ExpiresAt: time.Now().Add(time.Duration(60*24) * time.Hour)})
	t, _ := auth.MakeJWT(user.ID, cfg.secret, exp)

	respBody := LoginUser{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Token: t,
		Refresh: r.Token,
	}
	dat, _ := json.Marshal(respBody)
	rw.WriteHeader(200)
	rw.Write(dat)
}

func (cfg *apiConfig) deleteChirp(rw http.ResponseWriter, req *http.Request) {
	i := req.PathValue("chirpID")
	fmt.Printf("%s", i)
	chirpId, err := uuid.Parse(i)
	if err != nil {
		rw.WriteHeader(500)
		fmt.Printf("%s", err.Error())
		return
	}
	jwt, err1 := auth.GetBearerToken(req.Header)
	if err1 != nil {
		rw.WriteHeader(401)
		return
	}
	u, err2 := auth.ValidateJWT(jwt, cfg.secret)
	if err2 != nil {
		rw.WriteHeader(401)
		return
	}
	chirp, err := cfg.query.GetChirp(req.Context(), chirpId)
	if err != nil {
		rw.WriteHeader(404)
		return	
	}
	if chirp.UserID != u {
		rw.WriteHeader(403)
		return	
	}
	cfg.query.DeleteChirp(req.Context(), chirpId)
	rw.WriteHeader(204)
}

func (cfg *apiConfig) updateUser(rw http.ResponseWriter, req *http.Request) {
	type parameters struct {
		Password  string    `json:"password"`
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	jwt, err1 := auth.GetBearerToken(req.Header)
	if err1 != nil {
		rw.WriteHeader(401)
		return
	}
	u, err2 := auth.ValidateJWT(jwt,cfg.secret)
	if err2 != nil {
		rw.WriteHeader(401)
		return
	}
	err := decoder.Decode(&params)
	if err != nil {
		rw.WriteHeader(500)
		res := failVals{
			Err: "Something went wrong",
		}
		dat, _ := json.Marshal(res)
		rw.Write(dat)
		return
	}
	h, _ := auth.HashPassword(params.Password)
	user, err := cfg.query.UpdateUser(req.Context(), database.UpdateUserParams{HashedPassword: h, Email: params.Email, ID: u})
	if err != nil {
		rw.WriteHeader(500)
	}
	respBody := User{
		ID:        user.ID,
		Email:      user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	dat, _ := json.Marshal(respBody)
	rw.WriteHeader(200)
	rw.Write(dat)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, _ := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)
	apiCfg := apiConfig{query: dbQueries, env: os.Getenv("PLATFORM"), secret: os.Getenv("SECRET")}
	mux := http.NewServeMux()
	var filesSytems http.Dir = "."
	h := http.FileServer(filesSytems)
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(h))
	mux.Handle("/app/assets/", apiCfg.middlewareMetricsInc(h))
	mux.HandleFunc("GET /api/healthz", health)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/users", apiCfg.addUser)
	mux.HandleFunc("POST /api/chirps", apiCfg.chirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.chirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirp)
	mux.HandleFunc("POST /api/login", apiCfg.login)
	mux.HandleFunc("POST /api/refresh", apiCfg.refresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.revoke)
	mux.HandleFunc("PUT /api/users", apiCfg.updateUser)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirp)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	server.ListenAndServe()
}
