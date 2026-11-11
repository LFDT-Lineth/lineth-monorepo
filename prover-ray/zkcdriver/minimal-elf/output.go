package minimalelf

import "fmt"

// outputMaxN bounds [OutputELF]: the bytes are staged at base+1216 and their
// end is formed with one ADDI, whose signed 12-bit immediate stops at 2047.
const outputMaxN = 2047 - 'A'

// OutputELF returns a minimal valid ELF64 RISC-V binary that writes n bytes to
// guest_output in a single write_output call, together with those bytes: 'A',
// 'B', 'C', … in order (wrapping past 0xff). n may be zero, in which case the
// guest writes nothing.
func OutputELF(n int) (elf, output []byte) {
	if n < 0 || n > outputMaxN {
		panic(fmt.Sprintf("minimalelf: OutputELF n=%d outside [0, %d]", n, outputMaxN))
	}

	a := newAssembler()
	a.auipc(s0, 0)
	a.addi(t1, s0, 1216)           // cursor, clear of .text
	a.addi(t2, zero, 'A')          // byte to write
	a.addi(t3, zero, int32('A'+n)) // byte past the last one
	a.beq(t2, t3, "write")
	a.label("loop")
	a.sb(t2, t1, 0)
	a.addi(t1, t1, 1)
	a.addi(t2, t2, 1)
	a.bne(t2, t3, "loop")
	a.label("write")
	a.addi(t1, s0, 1216)
	a.addi(t3, zero, int32(n))
	a.writeOutput(t1, t3)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()

	output = make([]byte, n)
	for i := range output {
		output[i] = byte('A' + i)
	}
	return Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes()), output
}

// outputAndLogMaxN bounds each buffer of [OutputAndLogELF]: its bytes are
// stored with one SB each, whose signed 12-bit offset stops at 2047.
const outputAndLogMaxN = 2047

// OutputAndLogELF returns a minimal valid ELF64 RISC-V binary that writes
// output to guest_output and log to guest_log, each in a single write call,
// and exits 0. Each may be at most 2047 bytes.
func OutputAndLogELF(output, log []byte) []byte {
	if len(output) > outputAndLogMaxN || len(log) > outputAndLogMaxN {
		panic(fmt.Sprintf("minimalelf: OutputAndLogELF output %d or log %d bytes past %d",
			len(output), len(log), outputAndLogMaxN))
	}

	a := newAssembler()
	a.auipc(s0, 0)
	// Buffers clear of .text, which is 8 bytes per staged byte.
	a.lui(t4, 0x8)
	a.add(t1, s0, t4) // output buffer
	a.lui(t4, 0x9)
	a.add(t5, s0, t4) // log buffer
	stage := func(buf reg, data []byte) {
		for i, b := range data {
			a.addi(t2, zero, int32(b))
			a.sb(t2, buf, int32(i))
		}
	}
	stage(t1, output)
	stage(t5, log)
	a.addi(t3, zero, int32(len(output)))
	a.writeOutput(t1, t3)
	a.addi(t3, zero, int32(len(log)))
	a.writeLog(t5, t3)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()

	return Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes())
}
