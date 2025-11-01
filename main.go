package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"encoding/json"
	"strings"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
    return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
        cfg.fileserverHits.Add(1)
        next.ServeHTTP(rw, req)
    })
}
func health (rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	rw.Write([]byte("OK"))
}

func (cfg *apiConfig) metrics (rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "text/html")
	s := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())
	fmt.Fprintf(rw, "%s", s)
}

func (cfg *apiConfig) reset (rw http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	rw.WriteHeader(http.StatusOK)
}

func validateChirp (rw http.ResponseWriter, req *http.Request) {
	rw.Header().Add("Content-Type", "application/json")
    type failVals struct {
		Err string `json:"error"`
    }
    type successVals struct {
		Valid string `json:"cleaned_body"`
    }
    type parameters struct {
        Chirp string `json:"body"`
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

    respBody := successVals{
        Valid: strings.Join(n, " "),
    }
    dat, _ := json.Marshal(respBody)

    rw.Write(dat)
}

func main() {
	apiCfg := apiConfig{}
	mux := http.NewServeMux()
	var filesSytems http.Dir = "."
	h := http.FileServer(filesSytems)
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(h))
	mux.Handle("/app/assets/", apiCfg.middlewareMetricsInc(h))
	mux.HandleFunc("GET /api/healthz", health)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)


	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	server.ListenAndServe()
}
