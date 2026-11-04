package minimalelf

import (
	"encoding/binary"
	"fmt"
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

// memoryMaxN is the largest N for which the 8·N-byte array of [MemoryELF],
// starting 4096 bytes past the entry point, ends within the 32-bit guest
// address space.
const memoryMaxN = (1<<32 - DefaultEntryPoint - 4096) / 8

// MemoryELF returns a minimal valid ELF64 RISC-V binary that stores the
// 64-bit words 0, 1, …, N-1 to N consecutive doublewords, loads them all back
// and sums them, writing the 8-byte little-endian sum mod 2^64 to
// guest_output, together with that expected output. Every iteration of both
// loops touches a distinct address, so it makes more RAM traffic than
// [FibonacciELF]; the trace is still dominated by the per-instruction modules
// (registers, bitwise lookups), not by the RAM module. The dynamic
// instruction count is 9·N + 21 (4 per store iteration, 5 per load
// iteration). N must be in [1, memoryMaxN].
//
// The array starts 4096 bytes past the auipc base, clear of the .text and of
// the 8-byte result slot 768 bytes past the base.
func MemoryELF(n int) (elf, output []byte) {
	if n < 1 || n > memoryMaxN {
		panic(fmt.Sprintf("minimalelf: MemoryELF N=%d outside [1, %d]", n, memoryMaxN))
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
