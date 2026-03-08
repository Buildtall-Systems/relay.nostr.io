package grpc

import (
	"context"
	"encoding/hex"
	"log/slog"

	"github.com/nbd-wtf/go-nostr/nip19"

	"github.com/Buildtall-Systems/relay.nostr.io/internal/db"
	pb "github.com/Buildtall-Systems/relay.nostr.io/proto"
)

type Server struct {
	pb.UnimplementedAuthorizationServer
	db     *db.DB
	logger *slog.Logger
}

func NewServer(database *db.DB, logger *slog.Logger) *Server {
	return &Server{
		db:     database,
		logger: logger,
	}
}

func (s *Server) EventAdmit(ctx context.Context, req *pb.EventRequest) (*pb.EventReply, error) {
	if len(req.AuthPubkey) == 0 {
		s.logger.Info("DENY: no NIP-42 authentication")
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("auth-required: NIP-42 authentication required"),
		}, nil
	}

	if req.Event == nil {
		s.logger.Info("DENY: no event provided")
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("blocked: no event provided"),
		}, nil
	}

	authNpub, err := nip19.EncodePublicKey(hex.EncodeToString(req.AuthPubkey))
	if err != nil {
		s.logger.Error("encoding auth pubkey to npub", "err", err)
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("internal error"),
		}, nil
	}

	allowed, err := s.db.IsAllowedPubkey(ctx, authNpub)
	if err != nil {
		s.logger.Error("database lookup failed", "npub", authNpub, "err", err)
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("internal error"),
		}, nil
	}

	if !allowed {
		s.logger.Info("DENY: npub not authorized", "npub", authNpub)
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("your pubkey is not authorized to publish"),
		}, nil
	}

	eventNpub, _ := nip19.EncodePublicKey(hex.EncodeToString(req.Event.Pubkey))
	s.logger.Info("PERMIT",
		"auth_npub", authNpub,
		"event_npub", eventNpub,
		"kind", req.Event.Kind)

	return &pb.EventReply{
		Decision: pb.Decision_DECISION_PERMIT,
	}, nil
}

func strPtr(s string) *string {
	return &s
}
