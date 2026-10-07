package minimalelf

// allInOneOutput is what the all-in-one guest writes to guest_output when
// every check passes.
const allInOneOutput = "ABC"

// AllInOneElfProgram returns a minimal valid ELF64 RISC-V binary that
// exercises the full RV64I + M-extension + custom-precompile surface used by
// the RISC-V arithmetization in a single guest, together with the bytes the
// guest writes to guest_output on success. See [allInOneSectionData] for the
// program.
func AllInOneElfProgram() (elf, output []byte) {
	return Make(DefaultEntryPoint, DefaultSectionAddr, allInOneSectionData()), []byte(allInOneOutput)
}

// allInOneSectionData assembles a tiny valid RISC-V program that exercises
// the full RV64I + M-extension + custom-precompile surface used by the RISC-V
// arithmetization in a single guest: memory round-trip (SW/LW),
// LUI/JAL/JALR/ADD/MUL, all six B-type branch variants (taken and
// not-taken), SB/LB/SH/LH/SD/LD (with sign extension), the R_POSEIDON2 and
// R_KECCAK precompiles, R_WRITE_OUTPUT, every I-type ALU immediate (SLTI,
// SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI) plus the RV64 word-width I-type
// variants (ADDIW, SLLIW, SRLIW, SRAIW), and the RV64 R-type word-width
// variants (ADDW, SUBW, SLLW, SRLW, SRAW). It halts through the Linux-style
// exit syscall path used by the arithmetization, exiting with code 1 if any
// check disagrees with its independently-computed expected value.
//
// s0 is set once via auipc at the top and held as a fixed base for every
// scratch-memory offset below (each check would otherwise re-auipc at its
// own, different program-counter position and address a different section of
// memory than intended). Scratch regions (768, 800, 808, 816, 896, 960, 1088,
// 1216 bytes past that base) are laid out with enough spacing to stay
// non-overlapping and clear of the ~624-byte .text, while remaining within
// the 12-bit signed immediate range (max 2047) that S-type and I-type
// instructions can address in one step.
func allInOneSectionData() []byte {
	a := newAssembler()

	a.addi(a7, zero, 93) // exit syscall number, set once
	a.auipc(s0, 0)       // s0 = fixed base for every scratch offset below

	// ---- memory round trip: store/load word at base+768 ----
	a.addi(t1, zero, 42)
	a.sw(t1, s0, 768)
	a.lw(t2, s0, 768)
	a.bne(t1, t2, "fail")

	// ---- arithmetic: (t0+t1)*3 via subroutine, compare vs LUI-built const ----
	a.lui(t0, 1)
	a.addi(t1, zero, 5)
	a.jal(ra, "add_mul")
	a.bne(t3, t4, "fail")

	// ---- branches: all six B-type variants, taken and not-taken ----
	a.addi(t0, zero, 3)
	a.addi(t1, zero, -1)
	a.addi(t2, zero, 3)
	a.beq(t0, t2, "br_ok1")
	a.jal(zero, "fail")
	a.label("br_ok1")
	a.bne(t0, t1, "br_ok2")
	a.jal(zero, "fail")
	a.label("br_ok2")
	a.blt(t1, t0, "br_ok3")
	a.jal(zero, "fail")
	a.label("br_ok3")
	a.bge(t0, t1, "br_ok4")
	a.jal(zero, "fail")
	a.label("br_ok4")
	a.bltu(t0, t1, "br_ok5")
	a.jal(zero, "fail")
	a.label("br_ok5")
	a.bgeu(t1, t0, "br_ok6")
	a.jal(zero, "fail")
	a.label("br_ok6")
	a.beq(t0, t1, "fail")
	a.bne(t0, t2, "fail")
	a.blt(t0, t1, "fail")
	a.bge(t1, t0, "fail")
	a.bltu(t1, t0, "fail")
	a.bgeu(t0, t1, "fail")

	// ---- load/store widths: SB/LB, SH/LH, SD/LD at base+800/808/816 ----
	a.addi(t1, zero, 171)
	a.sb(t1, s0, 800)
	a.lb(t2, s0, 800) // t2 = sign_extend(0xAB) = -85
	a.addi(t3, zero, -85)
	a.bne(t2, t3, "fail")
	a.addi(t1, zero, -1)
	a.sh(t1, s0, 808)
	a.lh(t2, s0, 808) // t2 = sign_extend(0xFFFF) = -1
	a.addi(t3, zero, -1)
	a.bne(t2, t3, "fail")
	a.addi(t1, zero, 1000)
	a.sd(t1, s0, 816)
	a.ld(t2, s0, 816) // exact round trip, no extension
	a.bne(t2, t1, "fail")

	// ---- poseidon2: permute all-zero block at base+896, output at base+960 ----
	a.addi(t1, s0, 960) // output
	a.addi(t2, s0, 896) // input (still zeroed)
	a.poseidon2(t1, t2)
	a.lw(t3, t1, 0)
	a.lui(t4, 0x35596)
	a.addi(t4, t4, 407) // expected first output word
	a.bne(t3, t4, "fail")

	// ---- keccak: empty message, output at base+1088 ----
	a.addi(t1, s0, 1088) // output
	a.keccak(t1, s0, zero)
	a.lw(t3, t1, 0)
	a.lui(t4, 0x146d)
	a.addi(t4, t4, 709) // expected first digest word
	a.bne(t3, t4, "fail")

	// ---- write_output: write allInOneOutput to guest_output, scratch at base+1216 ----
	a.addi(t1, s0, 1216)
	for i, c := range []byte(allInOneOutput) {
		a.addi(t2, zero, int32(c))
		a.sb(t2, t1, int32(i))
	}
	a.addi(t3, zero, int32(len(allInOneOutput)))
	a.writeOutput(t1, t3)

	// ---- immediate ALU: SLTI, SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI,
	// plus RV64 ADDIW, SLLIW, SRLIW, SRAIW. t2=6 is the shared operand for
	// the base checks; SRAI uses t2=-8 to exercise sign-preserving shifts; the
	// *W checks reload t2 with 32-bit boundary values (0x7FFFFFFF,
	// 0xFFFFFFFF) to exercise word-width wraparound and sign extension.
	// Because LUI sign-extends its 32-bit result to 64 bits, loading a value
	// whose bit 31 is 0 but whose upper 20 bits (as loaded via LUI) form a
	// pattern with bit 31 set (e.g. 0x7FFFFFFF, via LUI 0x80000 + ADDI -1)
	// requires an extra SLLI 32 / SRLI 32 pair to zero the sign-extended
	// upper 32 bits afterward. SRLIW's comparison constant needs that same
	// masking even though t2 isn't reloaded: SRLIW zero-extends its 32-bit
	// result, unlike ADDIW/SLLIW/SRAIW which sign-extend, so the expected
	// constant (built via LUI+ADDI, which does sign-extend) must be masked
	// down to match. ----
	a.addi(t2, zero, 6)
	a.slti(t0, t2, 10) // expect 1 (6 < 10 signed)
	a.addi(t5, zero, 1)
	a.bne(t0, t5, "fail")
	a.sltiu(t0, t2, 10) // expect 1 (6 < 10 unsigned)
	a.addi(t5, zero, 1)
	a.bne(t0, t5, "fail")
	a.xori(t0, t2, 3) // expect 5 (6 ^ 3)
	a.addi(t5, zero, 5)
	a.bne(t0, t5, "fail")
	a.ori(t0, t2, 1) // expect 7 (6 | 1)
	a.addi(t5, zero, 7)
	a.bne(t0, t5, "fail")
	a.andi(t0, t2, 2) // expect 2 (6 & 2)
	a.addi(t5, zero, 2)
	a.bne(t0, t5, "fail")
	a.slli(t0, t2, 2) // expect 24 (6 << 2)
	a.addi(t5, zero, 24)
	a.bne(t0, t5, "fail")
	a.srli(t0, t2, 1) // expect 3 (6 >> 1)
	a.addi(t5, zero, 3)
	a.bne(t0, t5, "fail")
	a.addi(t2, zero, -8)
	a.srai(t0, t2, 1) // expect -4 (arithmetic shift preserves sign)
	a.addi(t5, zero, -4)
	a.bne(t0, t5, "fail")
	loadInt32Max(a, t2) // t2 = 0x7FFFFFFF
	a.addiw(t0, t2, 10) // expect sign_extend32(0x7FFFFFFF+10) (wraps at 32 bits)
	a.lui(t4, 0x80000)
	a.addi(t4, t4, 9)
	a.bne(t0, t4, "fail")
	a.addi(t2, zero, 6)
	a.slliw(t0, t2, 2) // expect 24 (word-width, still fits)
	a.addi(t5, zero, 24)
	a.bne(t0, t5, "fail")
	a.lui(t2, 0)
	a.addi(t2, t2, -1) // t2 = 0xFFFFFFFFFFFFFFFF (low32 = 0xFFFFFFFF)
	a.srliw(t0, t2, 1) // expect 0x7FFFFFFF (logical shift within 32 bits, zero-extended)
	loadInt32Max(a, t4)
	a.bne(t0, t4, "fail")
	a.sraiw(t0, t2, 1) // expect -1 (arithmetic shift of all-ones is still all-ones)
	a.addi(t5, zero, -1)
	a.bne(t0, t5, "fail")

	// ---- word width: ADDW, SUBW, SLLW, SRLW, SRAW, each operating on the
	// low 32 bits of its operands and sign-extending the 32-bit result to 64
	// bits (except SRLW, which zero-extends, same as SRLIW above). Operands
	// are chosen to force 32-bit wraparound (ADDW, SLLW) and sign-extension
	// of a negative 32-bit result (SUBW, SRAW). ----
	loadInt32Max(a, t2) // t2 = 0x7FFFFFFF
	a.addi(t3, zero, 1)
	a.addw(t0, t2, t3) // expect sign_extend32(0x7FFFFFFF+1) (wraps to INT32_MIN)
	a.lui(t4, 0x80000)
	a.bne(t0, t4, "fail")
	a.addi(t2, zero, 5)
	a.addi(t3, zero, 10)
	a.subw(t0, t2, t3) // expect -5 (word-width subtraction underflows, sign-extends)
	a.addi(t5, zero, -5)
	a.bne(t0, t5, "fail")
	a.addi(t2, zero, 1)
	a.addi(t3, zero, 31)
	a.sllw(t0, t2, t3) // expect sign_extend32(1<<31) (= INT32_MIN)
	a.lui(t4, 0x80000)
	a.bne(t0, t4, "fail")
	a.lui(t2, 0)
	a.addi(t2, t2, -1) // t2 = 0xFFFFFFFFFFFFFFFF (low32 = 0xFFFFFFFF)
	a.addi(t3, zero, 1)
	a.srlw(t0, t2, t3) // expect 0x7FFFFFFF (logical shift within 32 bits, zero-extended)
	loadInt32Max(a, t4)
	a.bne(t0, t4, "fail")
	a.sraw(t0, t2, t3) // expect -1 (arithmetic shift of all-ones is still all-ones)
	a.addi(t5, zero, -1)
	a.bne(t0, t5, "fail")

	// ---- exit(0) on success, exit(1) on any failed check ----
	a.addi(a0, zero, 0)
	a.ecall()
	a.label("fail")
	a.addi(a0, zero, 1)
	a.ecall()

	// add_mul: t3 = (t0+t1)*3, t4 = the expected value (0x1005*3 = 0x300F).
	a.label("add_mul")
	a.add(t2, t0, t1)
	a.addi(t3, zero, 3)
	a.mul(t3, t2, t3)
	a.lui(t4, 3)
	a.addi(t4, t4, 15)
	a.jalr(zero, ra, 0)

	return a.bytes()
}

// loadInt32Max sets rd = 0x7FFFFFFF (INT32_MAX): LUI 0x80000 + ADDI -1 yields
// 0xFFFFFFFF7FFFFFFF after LUI's sign extension, and SLLI/SRLI 32 clears the
// upper 32 bits.
func loadInt32Max(a *assembler, rd reg) {
	a.lui(rd, 0x80000)
	a.addi(rd, rd, -1)
	a.slli(rd, rd, 32)
	a.srli(rd, rd, 32)
}
