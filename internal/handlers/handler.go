package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Cookit-labs/Backend/internal/db"
	"github.com/Cookit-labs/Backend/internal/ws"
)

type Handler struct {
	db  *db.DB
	hub *ws.Hub
	// pubsub *ws.PubSub
}

// redisPubSub *ws.PubSub
func New(database *db.DB, wsHub *ws.Hub) *Handler {
	return &Handler{
		db:  database,
		hub: wsHub,
		// pubsub: redisPubSub,
	}
}

func (h *Handler) Mount(r chi.Router) {
	r.Post("/intents", h.CreateIntent)
	r.Get("/intents/{intentID}", h.GetIntent)
	r.Post("/intents/{intentID}/proposals", h.CreateProposal)
	r.Get("/intents/{intentID}/proposals", h.ListProposals)
	r.Post("/intents/{intentID}/select", h.SelectWinner)
	r.Post("/intents/{intentID}/validate", h.ValidateExecution)
	r.Post("/intents/{intentID}/settle", h.SettleExecution)
	r.Get("/leaderboard", h.GetLeaderboard)
	r.Get("/agents/{agentID}", h.GetAgent)
}

// Utility

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:  message,
		Status: status,
	})
}
