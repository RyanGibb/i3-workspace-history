{
  description = "Vim-like jumplist for i3/sway workspaces";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        ocamlPackages = pkgs.ocamlPackages;
      in
      {
        packages.default = ocamlPackages.buildDunePackage {
          pname = "i3-workspace-history";
          version = "1.0";
          src = ./.;
          buildInputs = with ocamlPackages; [ yojson cmdliner ];
        };

        devShells.default = pkgs.mkShell {
          packages = [
            ocamlPackages.ocaml
            ocamlPackages.dune_3
            ocamlPackages.yojson
            ocamlPackages.ocaml-lsp
            ocamlPackages.findlib
          ];
        };
      }
    );
}
