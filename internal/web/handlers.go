package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"

	"github.com/Buildtall-Systems/relay.nostr.io/internal/auth"
	"github.com/Buildtall-Systems/relay.nostr.io/views"
)

const sessionDuration = 24 * time.Hour

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		session, _ := s.sessions.Get(r.Context(), cookie.Value)
		if session != nil {
			admin, _ := s.db.IsAdmin(r.Context(), session.Npub)
			if admin {
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
		}
	}

	component := views.Login()
	if err := component.Render(r.Context(), w); err != nil {
		s.logger.Error("rendering login", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

type challengeRequest struct {
	Pubkey string `json:"pubkey"`
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	var req challengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Pubkey == "" {
		http.Error(w, "pubkey required", http.StatusBadRequest)
		return
	}

	npub, err := nip19.EncodePublicKey(req.Pubkey)
	if err != nil {
		http.Error(w, "invalid pubkey", http.StatusBadRequest)
		return
	}

	admin, err := s.db.IsAdmin(r.Context(), npub)
	if err != nil {
		s.logger.Error("checking admin", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !admin {
		http.Error(w, "not authorized", http.StatusForbidden)
		return
	}

	challenge, err := auth.NewChallenge(npub)
	if err != nil {
		s.logger.Error("creating challenge", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(challenge.Event)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var signedEvent nostr.Event
	if err := json.NewDecoder(r.Body).Decode(&signedEvent); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	npub, err := nip19.EncodePublicKey(signedEvent.PubKey)
	if err != nil {
		http.Error(w, "invalid pubkey", http.StatusBadRequest)
		return
	}

	if err := auth.VerifySignedChallenge(&signedEvent, npub); err != nil {
		http.Error(w, fmt.Sprintf("verification failed: %v", err), http.StatusUnauthorized)
		return
	}

	admin, err := s.db.IsAdmin(r.Context(), npub)
	if err != nil {
		s.logger.Error("checking admin", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !admin {
		http.Error(w, "not authorized", http.StatusForbidden)
		return
	}

	session, err := s.sessions.Create(r.Context(), npub, sessionDuration)
	if err != nil {
		s.logger.Error("creating session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		_ = s.sessions.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   auth.SessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	pubkeys, err := s.db.ListAllowedPubkeys(r.Context())
	if err != nil {
		s.logger.Error("listing pubkeys", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	component := views.Dashboard(pubkeys)
	if err := component.Render(r.Context(), w); err != nil {
		s.logger.Error("rendering dashboard", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleListPubkeys(w http.ResponseWriter, r *http.Request) {
	pubkeys, err := s.db.ListAllowedPubkeys(r.Context())
	if err != nil {
		s.logger.Error("listing pubkeys", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	component := views.PubkeyTable(pubkeys)
	if err := component.Render(r.Context(), w); err != nil {
		s.logger.Error("rendering pubkey table", "err", err)
	}
}

func (s *Server) handleAddPubkey(w http.ResponseWriter, r *http.Request) {
	session := auth.GetSession(r)

	npub := strings.TrimSpace(r.FormValue("npub"))
	note := strings.TrimSpace(r.FormValue("note"))

	if npub == "" {
		http.Error(w, "npub required", http.StatusBadRequest)
		return
	}

	if err := s.db.AddAllowedPubkey(r.Context(), npub, session.Npub, note); err != nil {
		s.logger.Error("adding pubkey", "err", err)
		http.Error(w, fmt.Sprintf("failed to add: %v", err), http.StatusBadRequest)
		return
	}

	s.handleListPubkeys(w, r)
}

func (s *Server) handleRemovePubkey(w http.ResponseWriter, r *http.Request) {
	npub := r.PathValue("npub")
	if npub == "" {
		http.Error(w, "npub required", http.StatusBadRequest)
		return
	}

	if err := s.db.RemoveAllowedPubkey(r.Context(), npub); err != nil {
		s.logger.Error("removing pubkey", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.handleListPubkeys(w, r)
}

type apiPubkeyEntry struct {
	Npub      string `json:"npub"`
	Note      string `json:"note"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

type apiAddPubkeyRequest struct {
	Npub string `json:"npub"`
	Note string `json:"note"`
}

func (s *Server) handleAPIListPubkeys(w http.ResponseWriter, r *http.Request) {
	pubkeys, err := s.db.ListAllowedPubkeys(r.Context())
	if err != nil {
		s.logger.Error("listing pubkeys", "err", err)
		writeAPIError(w, http.StatusInternalServerError, "internal error")
		return
	}

	entries := make([]apiPubkeyEntry, len(pubkeys))
	for i, pk := range pubkeys {
		entries[i] = apiPubkeyEntry{
			Npub:      pk.Npub,
			Note:      pk.Note,
			CreatedBy: pk.CreatedBy,
			CreatedAt: pk.CreatedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleAPIAddPubkey(w http.ResponseWriter, r *http.Request) {
	var req apiAddPubkeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Npub = strings.TrimSpace(req.Npub)
	if req.Npub == "" {
		writeAPIError(w, http.StatusBadRequest, "npub required")
		return
	}

	createdBy := auth.GetNIP98Npub(r)

	if err := s.db.AddAllowedPubkey(r.Context(), req.Npub, createdBy, req.Note); err != nil {
		s.logger.Error("adding pubkey", "err", err)
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("failed to add: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"npub":       req.Npub,
		"note":       req.Note,
		"created_by": createdBy,
	})
}

func (s *Server) handleAPIRemovePubkey(w http.ResponseWriter, r *http.Request) {
	npub := r.PathValue("npub")
	if npub == "" {
		writeAPIError(w, http.StatusBadRequest, "npub required")
		return
	}

	if err := s.db.RemoveAllowedPubkey(r.Context(), npub); err != nil {
		s.logger.Error("removing pubkey", "err", err)
		writeAPIError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
