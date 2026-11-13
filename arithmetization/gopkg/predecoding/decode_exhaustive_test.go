package predecoding

// Exhaustive check of the Go decoder on every 32-bit word.
//
// Claim: for each of the 2^32 words, decodeInstruction writes the record an
// independent reference says it should.
//   - Which words are instructions, and their operands (rd, rs1, rs2, imm),
//     come from riscv64asm, which is generated from the official riscv-opcodes
//     tables.
//   - compute_op numbers come from constants.zkc, the file the interpreter uses.
//   - The instruction -> compute_op mapping (spec table) and the x0-folding
//     rule are Lineth design choices, restated here. This test cannot tell
//     whether that design is right: what a compute_op *does* is checked by
//     executing programs (the ACT4 suite), not here.
//
// How this relates to ACT4. ACT4 runs a few dozen RISC-V certification programs
// through the decoder AND the interpreter, so it checks results, but only for
// the words those programs contain, and all of them are valid instructions.
// This test checks the decoder alone, on every word: other registers and
// immediates, and the malformed or reserved encodings ACT4 never feeds in. A
// failure here names the field that is wrong, where an ACT4 failure only says
// that some result is wrong. Each catches bugs the other misses.
//
// A failure names the instruction and the field that differs, with a sample word.

import (
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"

	"golang.org/x/arch/riscv64/riscv64asm"
)

// knownExtras lists the words the decoder accepts although they are not RV64IM
// instructions, with their exact counts. All are reserved or malformed
// encodings that no compiler emits. Any change to this set fails the test, so a
// new over-acceptance cannot slip in unnoticed.
//
// How the entries were derived: a full run with an empty map reports every
// "extra" group with its count. To regenerate, empty the map and rerun. The
// counts also follow from which bits each group leaves free:
//   - MISC-MEM fixes opcode (7 bits) and funct3 (3 bits), leaving 22 free bits,
//     so 2^22 words per funct3. funct3 = 000 is FENCE, a real instruction, so
//     it is not listed.
//   - ECALL/EBREAK fix every bit except rd and rs1 (5 + 5 free bits): 2^10 = 1024
//     words, minus the one valid encoding with rd = rs1 = 0, gives 1023 each.
var knownExtras = map[string]uint64{
	// MISC-MEM opcode with funct3 != 0 (FENCE.I, CBO.*, reserved) -> NO_OP.
	"extra: opcode=0001111 funct3=001 compute_op=0": 1 << 22,
	"extra: opcode=0001111 funct3=010 compute_op=0": 1 << 22,
	"extra: opcode=0001111 funct3=011 compute_op=0": 1 << 22,
	"extra: opcode=0001111 funct3=100 compute_op=0": 1 << 22,
	"extra: opcode=0001111 funct3=101 compute_op=0": 1 << 22,
	"extra: opcode=0001111 funct3=110 compute_op=0": 1 << 22,
	"extra: opcode=0001111 funct3=111 compute_op=0": 1 << 22,
	// ECALL/EBREAK with nonzero rd or rs1 -> still ECALL (23) / EBREAK (24).
	"extra: opcode=1110011 funct3=000 compute_op=23": 1023,
	"extra: opcode=1110011 funct3=000 compute_op=24": 1023,
}

type class int

const (
	cR     class = iota // register-register: rd, rs1, rs2
	cI                  // register-immediate: rd, rs1, signed imm
	cSH                 // shift by immediate: rd, rs1, unsigned shift amount
	cLOAD               // rd, imm(rs1)
	cJALR               // rd, imm(rs1); jump-only variant when rd == x0
	cSTORE              // rs2, imm(rs1)
	cBR                 // rs1, rs2, signed branch offset
	cJAL                // rd, signed jump offset; jump-only variant when rd == x0
	cU                  // rd, 20-bit upper immediate
	cSYS                // ecall/ebreak: imm holds bits 31..20
	cFENCE              // always NO_OP
)

// specEntry serves as the translation manual specifying how a valid RISC-V instruction should map
// to Lineth's internal instruction representation.
type specEntry struct {
	// constants.zkc name; for JAL/JALR the jump-only name (the _WB name is derived)
	name string

	cl class

	// rd == x0 turns the instruction into NO_OP
	fold bool
}

// spec maps every RV64IM mnemonic (as riscv64asm names it) to Lineth's
// compute_op. A mnemonic missing here is "not RV64IM", so the decoder must reject it.
var spec = map[string]specEntry{
	"LB": {"READ8_SGN_WB", cLOAD, true}, "LH": {"READ16_SGN_WB", cLOAD, true}, "LW": {"READ32_SGN_WB", cLOAD, true},
	"LD": {"READ64_WB", cLOAD, true}, "LBU": {"READ8_ZEXT_WB", cLOAD, true}, "LHU": {"READ16_ZEXT_WB", cLOAD, true},
	"LWU":  {"READ32_ZEXT_WB", cLOAD, true},
	"ADDI": {"OP_ADDI_WB", cI, true}, "SLTI": {"OP_SLTI_WB", cI, true}, "SLTIU": {"OP_SLTIU_WB", cI, true},
	"XORI": {"OP_XORI_WB", cI, true}, "ORI": {"OP_ORI_WB", cI, true}, "ANDI": {"OP_ANDI_WB", cI, true},
	"SLLI": {"OP_SLLI_WB", cSH, true}, "SRLI": {"OP_SRLI_WB", cSH, true}, "SRAI": {"OP_SRAI_WB", cSH, true},
	"ADDIW": {"OP_ADDIW_WB", cI, true}, "SLLIW": {"OP_SLLIW_WB", cSH, true}, "SRLIW": {"OP_SRLIW_WB", cSH, true},
	"SRAIW": {"OP_SRAIW_WB", cSH, true},
	"JALR":  {"ITYPE_JALR", cJALR, false}, "ECALL": {"ITYPE_ECALL", cSYS, false}, "EBREAK": {"ITYPE_EBREAK", cSYS, false},
	"ADD": {"RTYPE_ADD_WB", cR, true}, "SUB": {"RTYPE_SUB_WB", cR, true}, "SLL": {"RTYPE_SLL_WB", cR, true},
	"SLT": {"RTYPE_SLT_WB", cR, true}, "SLTU": {"RTYPE_SLTU_WB", cR, true}, "XOR": {"RTYPE_XOR_WB", cR, true},
	"SRL": {"RTYPE_SRL_WB", cR, true}, "SRA": {"RTYPE_SRA_WB", cR, true}, "OR": {"RTYPE_OR_WB", cR, true},
	"AND": {"RTYPE_AND_WB", cR, true}, "MUL": {"RTYPE_MUL_WB", cR, true}, "MULH": {"RTYPE_MULH_WB", cR, true},
	"MULHSU": {"RTYPE_MULHSU_WB", cR, true}, "MULHU": {"RTYPE_MULHU_WB", cR, true}, "DIV": {"RTYPE_DIV_WB", cR, true},
	"DIVU": {"RTYPE_DIVU_WB", cR, true}, "REM": {"RTYPE_REM_WB", cR, true}, "REMU": {"RTYPE_REMU_WB", cR, true},
	"ADDW": {"RTYPE_ADDW_WB", cR, true}, "SUBW": {"RTYPE_SUBW_WB", cR, true}, "SLLW": {"RTYPE_SLLW_WB", cR, true},
	"SRLW": {"RTYPE_SRLW_WB", cR, true}, "SRAW": {"RTYPE_SRAW_WB", cR, true}, "MULW": {"RTYPE_MULW_WB", cR, true},
	"DIVW": {"RTYPE_DIVW_WB", cR, true}, "DIVUW": {"RTYPE_DIVUW_WB", cR, true}, "REMW": {"RTYPE_REMW_WB", cR, true},
	"REMUW": {"RTYPE_REMUW_WB", cR, true},
	"SB":    {"STYPE_STORE8", cSTORE, false}, "SH": {"STYPE_STORE16", cSTORE, false}, "SW": {"STYPE_STORE32", cSTORE, false},
	"SD":  {"STYPE_STORE64", cSTORE, false},
	"BEQ": {"BRANCH_BEQ", cBR, false}, "BNE": {"BRANCH_BNE", cBR, false}, "BLT": {"BRANCH_BLT", cBR, false},
	"BGE": {"BRANCH_BGE", cBR, false}, "BLTU": {"BRANCH_BLTU", cBR, false}, "BGEU": {"BRANCH_BGEU", cBR, false},
	"JAL": {"JTYPE_JAL", cJAL, false},
	"LUI": {"UTYPE_LUI_WB", cU, true}, "AUIPC": {"UTYPE_AUIPC_WB", cU, true},
	"FENCE": {"NO_OP", cFENCE, false},
}

// custom1Names are Lineth's precompiles on the custom-1 opcode, indexed by funct3.
var custom1Names = []string{"RTYPE_KECCAK", "RTYPE_POSEIDON2"}

// TestDecodeAllWords decodes every 32-bit word and compares its record with the reference.
func TestDecodeAllWords(t *testing.T) {
	if testing.Short() {
		t.Skip("decodes all 2^32 words (~3 min on 14 cores); run without -short")
	}
	c := loadZkcConstants(t)

	type tally struct {
		n      uint64
		sample uint32 // first word seen in this category, for reproduction
	}

	// Each worker scans their assigned parts
	parts := forEachWord(func(lo, hi uint64) map[string]*tally {
		m := map[string]*tally{}
		var bw bitWriter
		for i := lo; i < hi; i++ {
			if k := checkWord(uint32(i), c, &bw); k != "" {
				if m[k] == nil {
					m[k] = &tally{sample: uint32(i)}
				}
				m[k].n++
			}
		}
		return m
	})

	// Merge all worker results
	all := map[string]*tally{}
	for _, m := range parts {
		for k, v := range m {
			if all[k] == nil {
				all[k] = &tally{sample: v.sample}
			}
			all[k].n += v.n
		}
	}

	// Check the results against known exceptions
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if want, ok := knownExtras[k]; ok && want == all[k].n {
			continue
		}
		t.Errorf("%s: count=%d (known: %d) e.g. %#08x", k, all[k].n, knownExtras[k], all[k].sample)
	}
	for k := range knownExtras {
		if all[k] == nil {
			t.Errorf("%s: known extra no longer accepted; update knownExtras", k)
		}
	}
}

// TestCheckWordDetectsWrongNumbering shows the exhaustive test can fail: with
// the ADD and SUB numbers swapped, `add x1, x2, x3` must be reported.
func TestCheckWordDetectsWrongNumbering(t *testing.T) {
	c := loadZkcConstants(t)
	var bw bitWriter
	const add = 0x003100b3 // add x1, x2, x3
	if k := checkWord(add, c, &bw); k != "" {
		t.Fatalf("baseline should pass, got %q", k)
	}
	c["RTYPE_ADD_WB"], c["RTYPE_SUB_WB"] = c["RTYPE_SUB_WB"], c["RTYPE_ADD_WB"]
	if checkWord(add, c, &bw) == "" {
		t.Fatal("swapped ADD/SUB numbers were not detected")
	}
}

// checkWord decodes x and compares the record with the reference. It returns
// "" when they agree, otherwise a category naming the disagreement. A word the
// decoder accepts but RV64IM does not define gets an "extra" category, which
// the caller compares with knownExtras.
func checkWord(x uint32, c map[string]uint64, bw *bitWriter) string {
	*bw = bitWriter{buf: bw.buf[:0]}
	decodeInstruction(bw, x)
	got := readRecord(bw.buf)
	want, name, defined := reference(x, c)
	switch {
	case !defined && got.op == computeInvalid:
		return ""
	case !defined:
		return fmt.Sprintf("extra: opcode=%07b funct3=%03b compute_op=%d", x&0x7f, (x>>12)&7, got.op)
	case got.op != want.op:
		return "wrong compute_op: " + name
	case want.op == computeNoOp: // NO_OP carries no operands
		return ""
	case got != want:
		return "wrong operands: " + name
	}
	return ""
}

// reference returns the record x should decode to, the instruction's name, and
// whether x is an instruction Lineth defines at all.
func reference(x uint32, c map[string]uint64) (rec, string, bool) {
	if x&3 != 3 { // 16-bit compressed encoding; Lineth has no C extension
		return rec{}, "", false
	}
	if x&0x7f == opcodeCUSTOM1 { // Lineth-only opcode: the reference knows nothing about it
		f3, f7 := (x>>12)&7, x>>25
		if f7 != 0 || int(f3) >= len(custom1Names) {
			return rec{}, "", false
		}
		r := rec{op: c[custom1Names[f3]], rs1: uint64((x >> 15) & 31), rs2: uint64((x >> 20) & 31), rd: uint64((x >> 7) & 31)}
		return r, custom1Names[f3], true // no x0 folding for precompiles
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], x)
	inst, err := riscv64asm.Decode(b[:])
	if err != nil {
		return rec{}, "", false
	}
	r, ok := expected(inst, x, c)
	return r, inst.Op.String(), ok
}

// expected builds the record for a word the reference decoded as inst:
// compute_op from the spec table and constants.zkc, operands from the
// reference. ok=false means the instruction is not RV64IM.
func expected(inst riscv64asm.Inst, x uint32, c map[string]uint64) (rec, bool) {
	e, ok := spec[inst.Op.String()]
	if !ok {
		return rec{}, false
	}
	rd := uint64((x >> 7) & 31)
	var r rec
	switch e.cl {
	case cFENCE:
		return rec{op: computeNoOp}, true
	case cJAL, cJALR: // rd == x0 means jump only; otherwise also save the return address
		r.op = c[e.name]
		if rd != 0 {
			r.op = c[e.name+"_WB"]
		}
	default:
		if e.fold && rd == 0 {
			return rec{op: computeNoOp}, true
		}
		r.op = c[e.name]
	}
	a := inst.Args
	switch e.cl {
	case cR:
		r.rd, r.rs1, r.rs2 = regNum(a[0]), regNum(a[1]), regNum(a[2])
	case cI:
		r.rd, r.rs1, r.imm = regNum(a[0]), regNum(a[1]), uint64(int64(a[2].(riscv64asm.Simm).Imm))
	case cSH:
		r.rd, r.rs1, r.imm = regNum(a[0]), regNum(a[1]), uint64(a[2].(riscv64asm.Uimm).Imm)
	case cLOAD, cJALR:
		o := a[1].(riscv64asm.RegOffset)
		r.rd, r.rs1, r.imm = regNum(a[0]), uint64(o.OfsReg-riscv64asm.X0), uint64(int64(o.Ofs.Imm))
	case cSTORE:
		o := a[1].(riscv64asm.RegOffset)
		r.rs2, r.rs1, r.imm = regNum(a[0]), uint64(o.OfsReg-riscv64asm.X0), uint64(int64(o.Ofs.Imm))
	case cBR:
		r.rs1, r.rs2, r.imm = regNum(a[0]), regNum(a[1]), uint64(int64(a[2].(riscv64asm.Simm).Imm))
	case cJAL:
		r.rd, r.imm = regNum(a[0]), uint64(int64(a[1].(riscv64asm.Simm).Imm))
	case cU:
		r.rd, r.imm = regNum(a[0]), uint64(int64(int32(a[1].(riscv64asm.Uimm).Imm<<12)))
	case cSYS:
		r.imm = uint64((x >> 20) & 0xfff)
	}
	return r, true
}

// loadZkcConstants reads `const NAME:ComputeOp = N` from constants.zkc, so the
// expected numbers come from the file the interpreter uses. It fails if a name
// the spec relies on is missing, so a renamed or removed op is reported as such
// instead of silently reading as 0 (NO_OP).
func loadZkcConstants(t *testing.T) map[string]uint64 {
	b, err := os.ReadFile("../../src/main/common/constants.zkc")
	if err != nil {
		t.Fatal(err)
	}
	c := map[string]uint64{}
	for _, g := range regexp.MustCompile(`(?m)^const (\w+):ComputeOp = (\d+)`).FindAllStringSubmatch(string(b), -1) {
		c[g[1]], _ = strconv.ParseUint(g[2], 10, 64)
	}
	// Every name the test looks up: the spec table's names, plus two kinds the
	// table doesn't list. The _WB variants of JAL/JALR are built by
	// appending "_WB" in expected(); only the jump-only names are in the table.
	// The custom-1 precompile names live in custom1Names, not in the table.
	need := append([]string{"ITYPE_JALR_WB", "JTYPE_JAL_WB"}, custom1Names...)
	for _, e := range spec {
		need = append(need, e.name)
	}
	for _, n := range need {
		if _, ok := c[n]; !ok {
			t.Fatalf("constants.zkc has no ComputeOp %s", n)
		}
	}
	return c
}

// rec is one decoded record, in the order decodeInstruction writes it.
type rec struct{ op, imm, rs1, rs2, rd uint64 }

// readRecord unpacks a record, most-significant bit first: op(8) imm(64) rs1(5) rs2(5) rd(5).
func readRecord(buf []byte) rec {
	pos := 0
	get := func(w int) uint64 {
		var v uint64
		for i := 0; i < w; i++ {
			v = v<<1 | uint64((buf[pos/8]>>(7-uint(pos%8)))&1)
			pos++
		}
		return v
	}
	return rec{get(8), get(64), get(5), get(5), get(5)}
}

// regNum converts a reference register argument to its number 0..31.
func regNum(a riscv64asm.Arg) uint64 { return uint64(a.(riscv64asm.Reg) - riscv64asm.X0) }

// forEachWord splits the 2^32 words into one contiguous range [lo, hi) per CPU,
// runs scan on each concurrently, and returns the per-range results. The last
// range ends exactly at 2^32, so no word is skipped.
func forEachWord[T any](scan func(lo, hi uint64) T) []T {
	workers := uint64(runtime.NumCPU())
	const total = uint64(1) << 32
	out := make([]T, workers)
	var wg sync.WaitGroup
	for w := uint64(0); w < workers; w++ {
		wg.Add(1)
		go func(w uint64) {
			defer wg.Done()
			out[w] = scan(total*w/workers, total*(w+1)/workers)
		}(w)
	}
	wg.Wait()
	return out
}
