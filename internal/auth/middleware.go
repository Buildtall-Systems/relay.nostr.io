package auth

import (
	"net/http"
)

const SessionCookieName = "relay_session"

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
