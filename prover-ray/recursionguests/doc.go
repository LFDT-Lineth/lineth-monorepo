// Package recursionguests holds the guests of the recursion machine (package
// recursion): the programs the machine proves, one package per guest, as
// riscv-guests holds the guests of the RISC-V zkVM. A guest is written
// against the machine's exported instructions and its in-machine verifiers
// (package recursion/verifier) only; the machine and the verifiers never
// import a guest, and a guest never imports the host that drives it.
//
// Guests:
//
//   - node: the node program of the conglomeration tree (its host is package
//     conglomeration).
package recursionguests
