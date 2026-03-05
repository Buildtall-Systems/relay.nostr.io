package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

func nip98Header(t *testing.T, sk, url, method string) string {
	t.Helper()
	ev := makeNIP98Event(t, sk, url, method, nostr.Now())
	jsonBytes, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	return "Nostr " + base64.StdEncoding.EncodeToString(jsonBytes)
}

func alwaysAdmin(_ string) (bool, error) { return true, nil }
func neverAdmin(_ string) (bool, error)  { return false, nil }

func TestRequireNIP98Admin(t *testing.T) {
	sk := nostr.GeneratePrivateKey()
	pub, _ := nostr.GetPublicKey(sk)
	baseURL := "http://localhost:8090"
	maxSkew := 60 * time.Second

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pk := GetNIP98Pubkey(r)
		if pk == "" {
			t.Error("expected pubkey in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	t.Run("valid admin request", func(t *testing.T) {
		path := "/api/v1/pubkeys"
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(ok)

		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", nip98Header(t, sk, baseURL+path, "GET"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing auth header", func(t *testing.T) {
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(ok)

		req := httptest.NewRequest("GET", "/api/v1/pubkeys", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("non-admin pubkey", func(t *testing.T) {
		path := "/api/v1/pubkeys"
		handler := RequireNIP98Admin(baseURL, maxSkew, neverAdmin)(ok)

		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", nip98Header(t, sk, baseURL+path, "GET"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("invalid auth header", func(t *testing.T) {
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(ok)

		req := httptest.NewRequest("GET", "/api/v1/pubkeys", nil)
		req.Header.Set("Authorization", "Bearer invalid")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("wrong URL in event", func(t *testing.T) {
		path := "/api/v1/pubkeys"
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(ok)

		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", nip98Header(t, sk, "http://evil.com/api/v1/pubkeys", "GET"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("wrong method in event", func(t *testing.T) {
		path := "/api/v1/pubkeys"
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(ok)

		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", nip98Header(t, sk, baseURL+path, "POST"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("pubkey injected in context", func(t *testing.T) {
		path := "/api/v1/pubkeys"
		var captured string
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured = GetNIP98Pubkey(r)
			w.WriteHeader(http.StatusOK)
		})
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(inner)

		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", nip98Header(t, sk, baseURL+path, "GET"))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if captured != pub {
			t.Fatalf("expected pubkey %s in context, got %s", pub, captured)
		}
	})

	t.Run("response is JSON on error", func(t *testing.T) {
		handler := RequireNIP98Admin(baseURL, maxSkew, alwaysAdmin)(ok)

		req := httptest.NewRequest("GET", "/api/v1/pubkeys", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		ct := rec.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}
	})
}
