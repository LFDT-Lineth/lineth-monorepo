{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.jdk17 pkgs.gradle_7 pkgs.solc pkgs.pre-commit
  ];
}
