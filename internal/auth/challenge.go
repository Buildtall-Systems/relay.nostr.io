package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const (
	ChallengeKind       = 22242
	ChallengeExpiration = 5 * time.Minute
)

type Challenge struct {
	Event     *nostr.Event
	CreatedAt time.Time
}

func NewChallenge(pubkey string) (*Challenge, error) {
	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, fmt.Errorf("generating challenge: %w", err)
	}

	event := &nostr.Event{
		PubKey:    pubkey,
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

func VerifySignedChallenge(signedEvent *nostr.Event, expectedPubkey string) error {
	if signedEvent.Kind != ChallengeKind {
		return fmt.Errorf("invalid event kind: expected %d, got %d", ChallengeKind, signedEvent.Kind)
	}

	if signedEvent.PubKey != expectedPubkey {
		return fmt.Errorf("pubkey mismatch: expected %s, got %s", expectedPubkey, signedEvent.PubKey)
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
