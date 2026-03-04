package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/Buildtall-Systems/relay.nostr.io/internal/auth"
	"github.com/Buildtall-Systems/relay.nostr.io/internal/config"
	"github.com/Buildtall-Systems/relay.nostr.io/internal/db"
	authzgrpc "github.com/Buildtall-Systems/relay.nostr.io/internal/grpc"
	pb "github.com/Buildtall-Systems/relay.nostr.io/proto"

	"github.com/Buildtall-Systems/relay.nostr.io/internal/web"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := &cobra.Command{
		Use:     "relay-authz",
		Short:   "Nostr relay authorization sidecar with admin webapp",
		Version: fmt.Sprintf("%s (%s) built %s", version, commit, date),
		RunE:    run,
	}

	root.Flags().String("config", "", "config file path")
	root.Flags().String("seed", "", "seed admins TOML file path")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	seedPath, _ := cmd.Flags().GetString("seed")

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))

	// Open database
	dbPath := filepath.Join(cfg.DatabaseDir, "relay-authz.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	logger.Info("database ready", "path", dbPath)

	// Seed admins if requested
	if seedPath != "" {
		if err := seedAdmins(database, seedPath); err != nil {
			return fmt.Errorf("seeding admins: %w", err)
		}
		logger.Info("admin seed complete", "file", seedPath)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	authzServer := authzgrpc.NewServer(database, logger)
	pb.RegisterAuthorizationServer(grpcServer, authzServer)

	grpcLis, err := net.Listen("tcp", cfg.GRPC.ListenAddress)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}

	go func() {
		logger.Info("gRPC server starting", "addr", cfg.GRPC.ListenAddress)
		errCh <- grpcServer.Serve(grpcLis)
	}()

	// Start HTTP server
	sessions := auth.NewSessionStore(database.DB)
	httpServer := web.New(cfg.HTTP.ListenAddress, database, sessions, logger)

	go func() {
		logger.Info("HTTP server starting", "addr", cfg.HTTP.ListenAddress)
		errCh <- httpServer.Start()
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		logger.Info("shutting down...")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
		}
		cancel()
	}

	// Graceful shutdown
	grpcServer.GracefulStop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5_000_000_000) // 5s
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)

	logger.Info("shutdown complete")
	return nil
}

func seedAdmins(database *db.DB, seedPath string) error {
	v := viper.New()
	v.SetConfigFile(seedPath)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("reading seed file: %w", err)
	}

	npubs := v.GetStringSlice("admin_npubs")
	if len(npubs) == 0 {
		return fmt.Errorf("no admin_npubs found in seed file")
	}

	return database.SeedAdmins(context.Background(), npubs)
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
