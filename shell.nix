{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  packages = with pkgs.ocamlPackages; [
    ocaml
    dune_3
    yojson
    cmdliner
    ocaml-lsp
    findlib
  ];
}
