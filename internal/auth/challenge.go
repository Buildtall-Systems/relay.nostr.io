package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const (
	ChallengeKind       = 22242
	ChallengeExpiration = 5 * time.Minute
)

type Challenge struct {
	Event     *nostr.Event
	CreatedAt time.Time
}

func NewChallenge(npub string) (*Challenge, error) {
	_, hexPubkey, err := nip19.Decode(npub)
	if err != nil {
		return nil, fmt.Errorf("decoding npub: %w", err)
	}

	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, fmt.Errorf("generating challenge: %w", err)
	}

	event := &nostr.Event{
		PubKey:    hexPubkey.(string),
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      ChallengeKind,
		Tags:      nostr.Tags{{"challenge", hex.EncodeToString(challengeBytes)}},
		Content:   "relay.nostr.io authentication challenge",
	}

	return &Challenge{
		Event:     event,
		CreatedAt: time.Now(),
	}, nil
}

func VerifySignedChallenge(signedEvent *nostr.Event, expectedNpub string) error {
	if signedEvent.Kind != ChallengeKind {
		return fmt.Errorf("invalid event kind: expected %d, got %d", ChallengeKind, signedEvent.Kind)
	}

	_, expectedHex, err := nip19.Decode(expectedNpub)
	if err != nil {
		return fmt.Errorf("decoding expected npub: %w", err)
	}

	if signedEvent.PubKey != expectedHex.(string) {
		return fmt.Errorf("pubkey mismatch")
	}

	eventTime := time.Unix(int64(signedEvent.CreatedAt), 0)
	if time.Since(eventTime) > ChallengeExpiration {
		return fmt.Errorf("challenge expired")
	}

	ok, err := signedEvent.CheckSignature()
	if err != nil {
		return fmt.Errorf("checking signature: %w", err)
	}
	if !ok {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
