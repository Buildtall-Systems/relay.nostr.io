{ self }:
{ config, lib, pkgs, ... }:

let
  cfg = config.services.relay-nostr-io;
  authzPkg = self.packages.${pkgs.system}.default;
in
{
  options.services.relay-nostr-io = {
    enable = lib.mkEnableOption "relay.nostr.io authenticated Nostr relay";

    relayPackage = lib.mkOption {
      type = lib.types.package;
      description = "The nostr-rs-relay package to use";
    };

    user = lib.mkOption {
      type = lib.types.str;
      default = "nostr-relay-io";
      description = "User under which services run";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "nostr-relay-io";
      description = "Group under which services run";
    };

    dataDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/relay.nostr.io";
      description = "Directory for relay and authz data";
    };

    relayPort = lib.mkOption {
      type = lib.types.port;
      default = 7778;
      description = "WebSocket port for nostr-rs-relay";
    };

    grpcAddress = lib.mkOption {
      type = lib.types.str;
      default = "[::1]:50052";
      description = "gRPC listen address for authz sidecar";
    };

    httpAddress = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1:8090";
      description = "HTTP listen address for admin webapp";
    };

    logLevel = lib.mkOption {
      type = lib.types.enum [ "DEBUG" "INFO" "WARN" "ERROR" ];
      default = "INFO";
      description = "Log level for authz service";
    };

    seedAdminNpubs = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [];
      description = "Admin npubs to seed on first run";
    };

    relaySettings = lib.mkOption {
      type = lib.types.attrsOf lib.types.anything;
      default = {};
      description = "Additional nostr-rs-relay config settings";
    };
  };

  config = lib.mkIf cfg.enable {
    users.users.${cfg.user} = {
      isSystemUser = true;
      group = cfg.group;
      home = cfg.dataDir;
      createHome = true;
      description = "relay.nostr.io service user";
    };

    users.groups.${cfg.group} = {};

    systemd.services.relay-nostr-io-authz = {
      description = "relay.nostr.io Authorization Sidecar";
      wantedBy = [ "multi-user.target" ];
      before = [ "relay-nostr-io.service" ];

      serviceConfig = {
        Type = "simple";
        User = cfg.user;
        Group = cfg.group;
        WorkingDirectory = cfg.dataDir;
        ExecStart = let
          authzConfig = pkgs.writeText "authz.toml" ''
            log_level = "${cfg.logLevel}"
            database_dir = "${cfg.dataDir}"

            [grpc]
            listen_address = "${cfg.grpcAddress}"

            [http]
            listen_address = "${cfg.httpAddress}"
          '';
          seedConfig = pkgs.writeText "seed-admins.toml" ''
            admin_npubs = [
            ${lib.concatMapStringsSep "\n" (npub: ''    "${npub}",'') cfg.seedAdminNpubs}
            ]
          '';
          seedFlag = lib.optionalString (cfg.seedAdminNpubs != []) " --seed ${seedConfig}";
        in "${authzPkg}/bin/relay-authz --config ${authzConfig}${seedFlag}";
        Restart = "always";
        RestartSec = 5;

        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        ReadWritePaths = [ cfg.dataDir ];
      };
    };

    systemd.services.relay-nostr-io = {
      description = "Nostr Relay (relay.nostr.io)";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" "relay-nostr-io-authz.service" ];
      requires = [ "relay-nostr-io-authz.service" ];

      serviceConfig = let
        relayConfig = pkgs.writeText "relay-config.toml" ''
          [info]
          relay_url = "wss://relay.nostr.io/"
          name = "relay.nostr.io"
          description = "Authenticated Nostr relay operated by buildtall.systems"

          [database]
          data_directory = "${cfg.dataDir}"

          [network]
          address = "0.0.0.0"
          port = ${toString cfg.relayPort}

          [grpc]
          event_admission_server = "http://${cfg.grpcAddress}"
          restricts_write = true

          [authorization]
          nip42_auth = true
        '';
      in {
        Type = "simple";
        User = cfg.user;
        Group = cfg.group;
        WorkingDirectory = cfg.dataDir;
        ExecStart = "${cfg.relayPackage}/bin/nostr-rs-relay --config ${relayConfig}";
        Restart = "always";
        RestartSec = 5;

        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        ReadWritePaths = [ cfg.dataDir ];
      };
    };
  };
}
