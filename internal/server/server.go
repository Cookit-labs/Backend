package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/Cookit-labs/Backend/internal/config"
	"github.com/Cookit-labs/Backend/internal/db"
	"github.com/Cookit-labs/Backend/internal/handlers"
	"github.com/Cookit-labs/Backend/internal/ws"
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true },
}

type Server struct {
	cfg    *config.Config
	db     *db.DB
	router *chi.Mux
	hub    *ws.Hub
	http   *http.Server
}

func New(cfg *config.Config, database *db.DB) *Server {
	s := &Server{
		cfg:  cfg,
		db:   database,
		hub:  ws.NewHub(),
	}
	s.router = s.buildRouter()
	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIPFromHeader("X-Forwarded-For"))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", s.handleHealth)
	r.Get("/ws/intents/{intentID}", s.handleWS)

	// API routes
	h := handlers.New(s.db, s.hub)
	r.Route("/api/v1", func(r chi.Router) {
		h.Mount(r)
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","time":"%s"}`, time.Now().UTC().Format(time.RFC3339))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "intentID")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := s.hub.Register(intentID, conn)
	s.hub.ReadPump(client)
}

func (s *Server) Hub() *ws.Hub { return s.hub }
func (s *Server) Router() *chi.Mux { return s.router }
func (s *Server) DB() *db.DB { return s.db }

func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
