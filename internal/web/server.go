package web

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Buildtall-Systems/relay.nostr.io/internal/auth"
	"github.com/Buildtall-Systems/relay.nostr.io/internal/db"
)

type Server struct {
	db       *db.DB
	sessions *auth.SessionStore
	logger   *slog.Logger
	server   *http.Server
}

func New(addr string, database *db.DB, sessions *auth.SessionStore, logger *slog.Logger) *Server {
	s := &Server{
		db:       database,
		sessions: sessions,
		logger:   logger,
	}

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /api/auth/challenge", s.handleChallenge)
	mux.HandleFunc("POST /api/auth/verify", s.handleVerify)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Admin routes
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /dashboard", s.handleDashboard)
	adminMux.HandleFunc("GET /api/pubkeys", s.handleListPubkeys)
	adminMux.HandleFunc("POST /api/pubkeys", s.handleAddPubkey)
	adminMux.HandleFunc("DELETE /api/pubkeys/{hex}", s.handleRemovePubkey)

	isAdmin := func(pubkeyHex string) (bool, error) {
		return database.IsAdmin(context.Background(), pubkeyHex)
	}
	mux.Handle("/", auth.RequireAdmin(sessions, isAdmin, adminMux))

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
