package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const SessionCookieName = "relay_session"

const nip98PubkeyKey contextKey = "nip98_pubkey"

func GetNIP98Pubkey(r *http.Request) string {
	v, _ := r.Context().Value(nip98PubkeyKey).(string)
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

			event, err := ParseNIP98FromHeader(authHeader)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid NIP-98 header: "+err.Error())
				return
			}

			expectedURL := publicBaseURL + r.URL.Path
			if err := VerifyNIP98Event(event, expectedURL, r.Method, maxSkew); err != nil {
				writeJSONError(w, http.StatusUnauthorized, "NIP-98 verification failed: "+err.Error())
				return
			}

			admin, err := isAdmin(event.PubKey)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal error")
				return
			}
			if !admin {
				writeJSONError(w, http.StatusForbidden, "pubkey not authorized as admin")
				return
			}

			ctx := context.WithValue(r.Context(), nip98PubkeyKey, event.PubKey)
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

		admin, err := isAdmin(session.PubkeyHex)
		if err != nil || !admin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		ctx := WithSession(r.Context(), session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
