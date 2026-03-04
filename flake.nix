{
  description = "relay.nostr.io — Authenticated Nostr Relay with Admin Webapp";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    btk-theme = {
      url = "git+file:///home/rob/git/buildtall.systems/tools/btk/themes/buildtall";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, flake-utils, btk-theme }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        theme = btk-theme.packages.${system}.default;
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_25
            gotools
            gopls
            go-tools
            golangci-lint
            templ
            tailwindcss_4
            air
            nodejs
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc
            grpcurl
          ];

          shellHook = ''
            export PROJECT_NAME="relay-authz"
            export PROJECT_MODULE="github.com/Buildtall-Systems/relay.nostr.io"
            export BTK_THEME_PATH="${theme}/css/buildtall-theme.css"
            echo "relay.nostr.io development environment"
            echo "   Go $(go version | awk '{print $3}')"
            echo "   templ $(templ version 2>/dev/null || echo 'installed')"
            echo ""
            echo "Quick commands:"
            echo "  make dev-web    - Run with live reload"
            echo "  make build      - Build binary"
            echo "  make test       - Run tests"
            echo "  make lint       - Run linter"
          '';
        };

        packages.default = pkgs.buildGoModule {
          pname = "relay-authz";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;

          nativeBuildInputs = with pkgs; [
            templ
            tailwindcss_4
          ];

          preBuild = ''
            export BTK_THEME_PATH="${theme}/css/buildtall-theme.css"
            templ generate
            cat $BTK_THEME_PATH static/css/custom.css > static/css/input.css
            tailwindcss -i static/css/input.css -o static/css/output.css --minify
          '';
        };
      }
    );
}
