package grpc

import (
	"context"
	"encoding/hex"
	"log/slog"

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
	if req.AuthPubkey == nil || len(req.AuthPubkey) == 0 {
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

	authPubkeyHex := hex.EncodeToString(req.AuthPubkey)

	allowed, err := s.db.IsAllowedPubkey(ctx, authPubkeyHex)
	if err != nil {
		s.logger.Error("database lookup failed", "pubkey", authPubkeyHex, "err", err)
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("internal error"),
		}, nil
	}

	if !allowed {
		s.logger.Info("DENY: pubkey not authorized", "pubkey", authPubkeyHex)
		return &pb.EventReply{
			Decision: pb.Decision_DECISION_DENY,
			Message:  strPtr("your pubkey is not authorized to publish"),
		}, nil
	}

	eventPubkeyHex := hex.EncodeToString(req.Event.Pubkey)
	s.logger.Info("PERMIT",
		"auth_pubkey", authPubkeyHex,
		"event_pubkey", eventPubkeyHex,
		"kind", req.Event.Kind)

	return &pb.EventReply{
		Decision: pb.Decision_DECISION_PERMIT,
	}, nil
}

func strPtr(s string) *string {
	return &s
}
