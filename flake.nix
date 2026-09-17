{
  description = "JKRouter — AI Routing Gateway devshell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs { inherit system; };
    in {
      devShells.default = pkgs.mkShell {
        name = "jkrouter-dev";

        packages = with pkgs; [
          # Go
          go_1_26
          gopls

          # Node / Nuxt / Bun
          nodejs_22
          bun

          # Utilities
          git
          direnv
        ];

        # direnv hook: auto-load .envrc on cd
        shellHook = ''
          export PATH="$HOME/.local/bin:$PATH"
          eval "$(direnv hook bash)"
          echo "🚀 JKRouter devshell active"
          echo "   Go   $(go version | awk '{print $3}')"
          echo "   Node $(node --version)"
          echo "   Bun  $(bun --version)"
        '';
      };
    });
}
