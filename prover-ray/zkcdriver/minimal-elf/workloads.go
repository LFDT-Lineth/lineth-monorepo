package minimalelf

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/sha3"
)

// loadConst emits LUI+ADDI materializing n in rd. n must be in
// [0, 0x7FFFF7FF], so that LUI's upper immediate never sets bit 31.
func (a *assembler) loadConst(rd reg, n int) {
	if n < 0 || n > fibonacciMaxN {
		panic(fmt.Sprintf("minimalelf: constant %d outside [0, %d]", n, fibonacciMaxN))
	}
	hi := uint32((n + 0x800) >> 12) // imm20 for LUI, with carry fixup
	lo := int32(n - int(hi)<<12)    // signed imm12 for ADDI
	a.lui(rd, hi)
	a.addi(rd, rd, lo)
}

// MemoryELF returns a minimal valid ELF64 RISC-V binary that stores the
// 64-bit words 0, 1, …, N-1 to N consecutive doublewords, loads them all back
// and sums them, writing the 8-byte little-endian sum mod 2^64 to
// guest_output, together with that expected output. It is a RAM-bound
// workload: every iteration of both loops touches a distinct address, so the
// memory modules grow with N while the ALU work stays trivial. The dynamic
// instruction count is about 9·N (4 per store iteration, 5 per load
// iteration). N must be in [1, 0x7FFFF7FF].
//
// The array starts 4096 bytes past the auipc base, clear of the .text and of
// the 8-byte result slot 768 bytes past the base.
func MemoryELF(n int) (elf, output []byte) {
	if n < 1 {
		panic(fmt.Sprintf("minimalelf: MemoryELF N=%d must be at least 1", n))
	}
	a := newAssembler()
	a.auipc(s0, 0) // s0 = fixed base (entry address)
	a.loadConst(t3, n)
	a.lui(t6, 1)
	a.add(t5, s0, t6) // t5 = array base = s0 + 4096

	// Store loop: mem[t5 + 8i] = i.
	a.addi(t2, zero, 0) // i = 0
	a.addi(t4, t5, 0)   // p = base
	a.label("store")
	a.sd(t2, t4, 0)
	a.addi(t4, t4, 8)
	a.addi(t2, t2, 1)
	a.bne(t2, t3, "store")

	// Load loop: sum += mem[t5 + 8i].
	a.addi(t2, zero, 0) // i = 0
	a.addi(t4, t5, 0)   // p = base
	a.addi(t1, zero, 0) // sum = 0
	a.label("load")
	a.ld(t0, t4, 0)
	a.add(t1, t1, t0)
	a.addi(t4, t4, 8)
	a.addi(t2, t2, 1)
	a.bne(t2, t3, "load")

	a.sd(t1, s0, 768)
	a.addi(t1, s0, 768) // ptr
	a.addi(t2, zero, 8) // len
	a.writeOutput(t1, t2)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()

	var sum uint64
	for i := range uint64(n) {
		sum += i
	}
	output = binary.LittleEndian.AppendUint64(nil, sum)
	return Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes()), output
}

// KeccakELF returns a minimal valid ELF64 RISC-V binary that hashes a 32-byte
// buffer with the R_KECCAK precompile 2·N times, each digest being the next
// message (h_0 = 32 zero bytes, h_{k+1} = keccak256(h_k)), and writes the
// final 32-byte digest to guest_output, together with that expected output.
// It is a precompile-bound workload: each loop iteration executes 4
// instructions but two Keccak permutations, so the Keccak modules dominate
// the trace. N must be in [1, 0x7FFFF7FF].
//
// The two digest buffers ping-pong between 768 and 832 bytes past the auipc
// base, clear of the .text.
func KeccakELF(n int) (elf, output []byte) {
	if n < 1 {
		panic(fmt.Sprintf("minimalelf: KeccakELF N=%d must be at least 1", n))
	}
	a := newAssembler()
	a.auipc(s0, 0) // s0 = fixed base (entry address)
	a.loadConst(t3, n)
	a.addi(t1, s0, 768) // buffer A (h_0 = zeros)
	a.addi(t4, s0, 832) // buffer B
	a.addi(t5, zero, 32)
	a.addi(t2, zero, 0) // i = 0
	a.label("loop")
	a.keccak(t4, t1, t5) // B = keccak(A)
	a.keccak(t1, t4, t5) // A = keccak(B)
	a.addi(t2, t2, 1)
	a.bne(t2, t3, "loop")

	a.writeOutput(t1, t5)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()

	h := make([]byte, 32)
	for range 2 * n {
		k := sha3.NewLegacyKeccak256()
		k.Write(h)
		h = k.Sum(nil)
	}
	return Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes()), h
}
