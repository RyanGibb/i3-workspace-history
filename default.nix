{ pkgs ? import <nixpkgs> {} }:

pkgs.ocamlPackages.buildDunePackage {
  pname = "i3-workspace-history";
  version = "1.0";
  src = ./.;
  buildInputs = with pkgs.ocamlPackages; [ yojson cmdliner ];
}
