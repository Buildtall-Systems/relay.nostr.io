package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

func makeNIP98Event(t *testing.T, sk string, url, method string, createdAt nostr.Timestamp) *nostr.Event {
	t.Helper()
	pub, err := nostr.GetPublicKey(sk)
	if err != nil {
		t.Fatal(err)
	}
	ev := &nostr.Event{
		PubKey:    pub,
		CreatedAt: createdAt,
		Kind:      KindHTTPAuth,
		Content:   "",
		Tags: nostr.Tags{
			{"u", url},
			{"method", method},
		},
	}
	if err := ev.Sign(sk); err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestVerifyNIP98Event(t *testing.T) {
	sk := nostr.GeneratePrivateKey()
	url := "http://localhost:8090/api/v1/pubkeys"
	method := "GET"
	now := nostr.Now()

	t.Run("valid event", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, method, now)
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err != nil {
			t.Fatalf("expected valid event, got: %v", err)
		}
	})

	t.Run("wrong kind", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, method, now)
		ev.Kind = 1
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for wrong kind")
		}
	})

	t.Run("non-empty content", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, method, now)
		ev.Content = "should be empty"
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for non-empty content")
		}
	})

	t.Run("URL mismatch", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, method, now)
		if err := VerifyNIP98Event(ev, "http://other.host/path", method, 60*time.Second); err == nil {
			t.Fatal("expected error for URL mismatch")
		}
	})

	t.Run("method mismatch", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, method, now)
		if err := VerifyNIP98Event(ev, url, "POST", 60*time.Second); err == nil {
			t.Fatal("expected error for method mismatch")
		}
	})

	t.Run("timestamp too old", func(t *testing.T) {
		old := nostr.Timestamp(time.Now().Add(-5 * time.Minute).Unix())
		ev := makeNIP98Event(t, sk, url, method, old)
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for stale timestamp")
		}
	})

	t.Run("timestamp in future", func(t *testing.T) {
		future := nostr.Timestamp(time.Now().Add(5 * time.Minute).Unix())
		ev := makeNIP98Event(t, sk, url, method, future)
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for future timestamp")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, method, now)
		// Corrupt the signature
		ev.Sig = "0000000000000000000000000000000000000000000000000000000000000000" +
			"0000000000000000000000000000000000000000000000000000000000000000"
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for invalid signature")
		}
	})

	t.Run("missing u tag", func(t *testing.T) {
		pub, _ := nostr.GetPublicKey(sk)
		ev := &nostr.Event{
			PubKey:    pub,
			CreatedAt: now,
			Kind:      KindHTTPAuth,
			Content:   "",
			Tags:      nostr.Tags{{"method", method}},
		}
		if err := ev.Sign(sk); err != nil {
			t.Fatal(err)
		}
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for missing u tag")
		}
	})

	t.Run("missing method tag", func(t *testing.T) {
		pub, _ := nostr.GetPublicKey(sk)
		ev := &nostr.Event{
			PubKey:    pub,
			CreatedAt: now,
			Kind:      KindHTTPAuth,
			Content:   "",
			Tags:      nostr.Tags{{"u", url}},
		}
		if err := ev.Sign(sk); err != nil {
			t.Fatal(err)
		}
		if err := VerifyNIP98Event(ev, url, method, 60*time.Second); err == nil {
			t.Fatal("expected error for missing method tag")
		}
	})

	t.Run("case-insensitive method match", func(t *testing.T) {
		ev := makeNIP98Event(t, sk, url, "get", now)
		if err := VerifyNIP98Event(ev, url, "GET", 60*time.Second); err != nil {
			t.Fatalf("expected case-insensitive method match, got: %v", err)
		}
	})
}

func TestParseNIP98FromHeader(t *testing.T) {
	sk := nostr.GeneratePrivateKey()
	ev := makeNIP98Event(t, sk, "http://localhost/test", "GET", nostr.Now())

	t.Run("valid header", func(t *testing.T) {
		jsonBytes, _ := json.Marshal(ev)
		header := "Nostr " + base64.StdEncoding.EncodeToString(jsonBytes)

		parsed, err := ParseNIP98FromHeader(header)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed.PubKey != ev.PubKey {
			t.Fatalf("pubkey mismatch: expected %s, got %s", ev.PubKey, parsed.PubKey)
		}
		if parsed.Kind != KindHTTPAuth {
			t.Fatalf("kind mismatch: expected %d, got %d", KindHTTPAuth, parsed.Kind)
		}
	})

	t.Run("missing Nostr prefix", func(t *testing.T) {
		if _, err := ParseNIP98FromHeader("Bearer abc123"); err == nil {
			t.Fatal("expected error for missing Nostr prefix")
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		if _, err := ParseNIP98FromHeader("Nostr !!!not-base64!!!"); err == nil {
			t.Fatal("expected error for invalid base64")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		header := "Nostr " + base64.StdEncoding.EncodeToString([]byte("not json"))
		if _, err := ParseNIP98FromHeader(header); err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("empty header", func(t *testing.T) {
		if _, err := ParseNIP98FromHeader(""); err == nil {
			t.Fatal("expected error for empty header")
		}
	})
}
