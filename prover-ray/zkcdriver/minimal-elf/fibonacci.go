package minimalelf

import (
	"encoding/binary"
	"fmt"
)

// fibonacciMaxN is the largest N that LUI+ADDI materializes as a positive
// 32-bit constant: beyond it LUI's upper immediate would set bit 31 and the
// result would be sign-extended.
const fibonacciMaxN = 0x7FFFF7FF

// FibonacciELF returns a minimal valid ELF64 RISC-V binary computing fib(N)
// mod 2^64 (wrapping u64 addition), writing the 8-byte little-endian result
// to guest_output and exiting 0, together with that expected output. N must
// be in [0, 0x7FFFF7FF]. The dynamic instruction count is 14 + 5*N (6 setup
// + guard, 5 per loop iteration, 7 epilogue), which makes the program useful
// as a predictable sharding benchmark: at 500K interpreter invocations per
// shard, N = 4M (the cross-zkVM Fibonacci workload) yields 40 full shards
// plus a 14-instruction tail shard.
//
// The result is stored in scratch memory 768 bytes past the auipc base,
// clear of the 76-byte .text, as in [allInOneSectionData].
func FibonacciELF(n int) (elf, output []byte) {
	if n < 0 || n > fibonacciMaxN {
		panic(fmt.Sprintf("minimalelf: FibonacciELF N=%d outside [0, %d]", n, fibonacciMaxN))
	}
	var (
		hi = uint32((n + 0x800) >> 12) // imm20 for LUI, with carry fixup
		lo = int32(n - int(hi)<<12)    // signed imm12 for ADDI
		a  = newAssembler()
	)
	a.auipc(s0, 0) // s0 = fixed base (entry address)
	a.lui(t3, hi)
	a.addi(t3, t3, lo)  // t3 = N
	a.addi(t0, zero, 0) // a = 0
	a.addi(t1, zero, 1) // b = 1
	a.addi(t2, zero, 0) // i = 0
	a.beq(t2, t3, "done")

	a.label("loop")
	a.add(t4, t0, t1) // t = a + b
	a.addi(t0, t1, 0) // a = b
	a.addi(t1, t4, 0) // b = t
	a.addi(t2, t2, 1) // i++
	a.bne(t2, t3, "loop")

	a.label("done")
	a.sd(t0, s0, 768)   // scratch = a = fib(N)
	a.addi(t1, s0, 768) // ptr
	a.addi(t2, zero, 8) // len
	a.writeOutput(t1, t2)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0), Linux-style syscall path
	a.ecall()

	var fa, fb uint64 = 0, 1
	for range n {
		fa, fb = fb, fa+fb
	}
	output = binary.LittleEndian.AppendUint64(nil, fa)
	return Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes()), output
}
