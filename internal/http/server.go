package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/binn/ledger-payments-api/internal/ledger"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	svc *ledger.Service
}

func New(svc *ledger.Service) http.Handler {
	s := &Server{svc: svc}
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Route("/v1", func(r chi.Router) {
		r.Post("/accounts", s.createAccount)
		r.Get("/accounts/{id}", s.getAccount)
		r.Post("/transfers", s.createTransfer)
	})
	return r
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var body struct{ Owner, Currency string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	a, err := s.svc.CreateAccount(r.Context(), body.Owner, body.Currency)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 201, a)
}

func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	a, err := s.svc.GetAccount(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, ledger.ErrNotFound) {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, a)
}

func (s *Server) createTransfer(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		http.Error(w, "Idempotency-Key header required", 400)
		return
	}
	var t ledger.Transfer
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	out, err := s.svc.Transfer(r.Context(), key, t)
	switch {
	case errors.Is(err, ledger.ErrInsufficientFunds):
		http.Error(w, err.Error(), 422)
	case errors.Is(err, ledger.ErrCurrencyMismatch):
		http.Error(w, err.Error(), 422)
	case errors.Is(err, ledger.ErrIdempotencyConflict):
		http.Error(w, err.Error(), 409)
	case err != nil:
		http.Error(w, err.Error(), 500)
	default:
		writeJSON(w, 201, out)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
