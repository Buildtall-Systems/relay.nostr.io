package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Buildtall-Systems/btk/auth/nip98"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const SessionCookieName = "relay_session"

const nip98NpubKey contextKey = "nip98_npub"

func GetNIP98Npub(r *http.Request) string {
	v, _ := r.Context().Value(nip98NpubKey).(string)
	return v
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{Error: msg})
}

func RequireNIP98Admin(publicBaseURL string, maxSkew time.Duration, isAdmin func(string) (bool, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			event, err := nip98.ParseNIP98FromHeader(authHeader)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid NIP-98 header: "+err.Error())
				return
			}

			expectedURL := publicBaseURL + r.URL.Path
			if err := nip98.VerifyNIP98Event(event, expectedURL, r.Method, maxSkew); err != nil {
				writeJSONError(w, http.StatusUnauthorized, "NIP-98 verification failed: "+err.Error())
				return
			}

			npub, err := nip19.EncodePublicKey(event.PubKey)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal error")
				return
			}

			admin, err := isAdmin(npub)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !admin {
				writeJSONError(w, http.StatusForbidden, "pubkey not authorized as admin")
				return
			}

			ctx := context.WithValue(r.Context(), nip98NpubKey, npub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(sessions *SessionStore, isAdmin func(string) (bool, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		session, err := sessions.Get(r.Context(), cookie.Value)
		if err != nil || session == nil {
			http.SetCookie(w, &http.Cookie{
				Name:   SessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		admin, err := isAdmin(session.Npub)
		if err != nil || !admin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		ctx := WithSession(r.Context(), session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
