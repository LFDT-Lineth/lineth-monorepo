// Package recursionguests holds the guests of the recursion machine (package
// recursion): the programs the machine proves, one package per guest, as
// riscv-guests holds the guests of the RISC-V zkVM. A guest is written
// against the machine's exported instructions and the in-machine verifier
// contract (package verifier, here) only; the machine never imports a guest,
// and a guest never imports the host that drives it.
//
// Packages:
//
//   - verifier: the contract a guest verifies a child proof through, with its
//     mock; the real in-machine wiop verifier implements it (package
//     recursion's side).
//   - node: the node program of the conglomeration tree (its host is package
//     conglomeration).
package recursionguests
