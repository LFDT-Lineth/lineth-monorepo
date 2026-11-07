package predecoding

import (
	"encoding/binary"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"golang.org/x/arch/riscv64/riscv64asm"
)

// Question 2 — is the Go decoder right? Two tests against an independent
// reference (riscv64asm, generated from riscv-opcodes):
//   - TestDiffRefRVOP: does Lineth accept exactly the RV64IM instructions, and
//     which non-instructions does it accept (grouped by opcode and funct3)?
//   - TestRecordsAgainstSpec: is every word's full record (compute_op, imm,
//     rs1, rs2, rd) right — numbering from constants.zkc (what the interpreter
//     executes), operands from the reference, and the documented x0 folding?
// The spec table below doubles as the definition of "valid RV64IM".

type class int

const (
	cR class = iota
	cI
	cSH
	cLOAD
	cJALR
	cSTORE
	cBR
	cJAL
	cU
	cSYS
	cFENCE
)

type specEntry struct {
	name string // constants.zkc name; JAL/JALR use base name, WB variant derived
	cl   class
	fold bool // rd==x0 folds to NO_OP
}

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

// loadZkcConstants parses `const NAME:ComputeOp = N` lines from constants.zkc
// into name -> compute_op number, so the expected numbering comes from the same
// file the interpreter uses rather than from Go constants.
func loadZkcConstants(t *testing.T) map[string]uint64 {
	b, err := os.ReadFile("../../src/main/common/constants.zkc")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^const (\w+):ComputeOp = (\d+)`)
	m := map[string]uint64{}
	for _, g := range re.FindAllStringSubmatch(string(b), -1) {
		v, _ := strconv.ParseUint(g[2], 10, 64)
		m[g[1]] = v
	}
	return m
}

// rec is one decoded instruction record, in the order decodeInstruction writes it.
type rec struct{ op, imm, rs1, rs2, rd uint64 }

// readRecord unpacks a bit-packed record, MSB first: op(8) imm(64) rs1(5) rs2(5) rd(5).
func readRecord(buf []byte) rec {
	pos := 0
	get := func(w int) uint64 {
		var v uint64
		for i := 0; i < w; i++ {
			bit := (buf[pos/8] >> (7 - uint(pos%8))) & 1
			v = v<<1 | uint64(bit)
			pos++
		}
		return v
	}
	return rec{get(8), get(64), get(5), get(5), get(5)}
}

// regNum converts a reference-decoder register argument to its number 0..31.
func regNum(a riscv64asm.Arg) uint64 { return uint64(a.(riscv64asm.Reg) - riscv64asm.X0) }

// expected builds the record Lineth should produce for a word the reference
// decoded as `inst`: compute_op from the spec table + constants.zkc, operands
// from the reference's decoded args. ok=false means the mnemonic isn't RV64IM.
// Rules encoded here:
//   - FENCE, and any writeback op with rd==x0 (fold=true), becomes NO_OP (0)
//   - JAL/JALR use the plain op when rd==x0 and the *_WB variant otherwise
//   - immediates are sign-extended to 64 bits; LUI/AUIPC shift the 20-bit field left by 12
func expected(inst riscv64asm.Inst, x uint32, c map[string]uint64) (rec, bool) {
	e, ok := spec[inst.Op.String()]
	if !ok {
		return rec{}, false
	}
	rd := uint64((x >> 7) & 31)
	var r rec
	switch e.cl {
	case cFENCE:
		return rec{op: 0}, true
	case cJAL:
		r.op = c["JTYPE_JAL"]
		if rd != 0 {
			r.op = c["JTYPE_JAL_WB"]
		}
	case cJALR:
		r.op = c["ITYPE_JALR"]
		if rd != 0 {
			r.op = c["ITYPE_JALR_WB"]
		}
	default:
		r.op = c[e.name]
		if e.fold && rd == 0 {
			return rec{op: 0}, true
		}
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

// Lineth-specific custom-1 spec (decode.go doc + constants.zkc): funct7 == 0,
// funct3 0/1/2 -> KECCAK/POSEIDON2/WRITE_OUTPUT with R-type operands, no x0 fold.
func expectedCustom1(x uint32, c map[string]uint64) rec {
	f3, f7 := (x>>12)&7, x>>25
	names := []string{"RTYPE_KECCAK", "RTYPE_POSEIDON2", "RTYPE_WRITE_OUTPUT"}
	if f7 != 0 || f3 > 2 {
		return rec{op: 255, rs1: uint64((x >> 15) & 31), rs2: uint64((x >> 20) & 31), rd: uint64((x >> 7) & 31)}
	}
	return rec{op: c[names[f3]], rs1: uint64((x >> 15) & 31), rs2: uint64((x >> 20) & 31), rd: uint64((x >> 7) & 31)}
}

// For all 2^32 words, runs Lineth's decodeInstruction and checks the full
// output record (compute_op, imm, rs1, rs2, rd) against the independent spec
// built by expected/expectedCustom1. Unlike the other tests it FAILS (t.Errorf)
// if any mismatch category is found.
func TestRecordsAgainstSpec(t *testing.T) {
	// compute_op numbers as the interpreter sees them, parsed from constants.zkc.
	c := loadZkcConstants(t)
	if os.Getenv("MUTATE") == "1" { // negative control: swap ADD/SUB expectations
		// Run with MUTATE=1: the test must now fail, proving it can detect a wrong mapping.
		a, b := spec["ADD"], spec["SUB"]
		a.name, b.name = b.name, a.name
		spec["ADD"], spec["SUB"] = a, b
	}
	t.Logf("constants.zkc: %d ComputeOp constants", len(c))
	// res is one worker's partial result, merged after all workers finish.
	type res struct {
		checked    uint64            // RV64IM words compared against the spec
		opMis      uint64            // words whose compute_op differs from the spec
		operandMis uint64            // words with the right compute_op but wrong imm/rs1/rs2/rd
		custom     uint64            // custom-1 words seen (checked separately)
		cats       map[string]uint64 // mismatch counts per category (e.g. "op:ADD")
		samples    map[string]string // first offending word per category, for reproduction
	}
	results := forEachWord(func(lo, hi uint64) res {
		r := res{cats: map[string]uint64{}, samples: map[string]string{}}
		var b [4]byte
		var bw bitWriter
		// note counts a mismatch under category k and keeps the first sample s.
		note := func(k, s string) {
			r.cats[k]++
			if _, ok := r.samples[k]; !ok {
				r.samples[k] = s
			}
		}
		for i := lo; i < hi; i++ {
			x := uint32(i)
			// 16-bit compressed encodings are out of scope here (see TestDiffRefRVOP).
			if x&3 != 3 {
				continue
			}
			// Lineth's output: decodeInstruction serializes the record into a bit
			// buffer (reused across words); readRecord unpacks it.
			bw = bitWriter{buf: bw.buf[:0]}
			decodeInstruction(&bw, x)
			got := readRecord(bw.buf)
			// Lineth-specific custom-1 opcode: the reference decoder can't help,
			// so compare against the hand-written expectedCustom1 spec.
			if x&0x7f == opcodeCUSTOM1 {
				r.custom++
				want := expectedCustom1(x, c)
				// want.op == 255 is the invalid marker, which carries no operand check.
				if got.op != want.op {
					note("custom1-op", fmt.Sprintf("%#08x got=%d want=%d", x, got.op, want.op))
				} else if want.op != 255 && (got.imm != 0 || got.rs1 != want.rs1 || got.rs2 != want.rs2 || got.rd != want.rd) {
					note("custom1-operands", fmt.Sprintf("%#08x got=%+v want=%+v", x, got, want))
				}
				continue
			}
			// Standard opcodes: decode with the reference. Words it rejects, and
			// mnemonics outside the spec table (not RV64IM), are skipped; their
			// accept/reject verdict is covered by TestDiffRefRVOP.
			binary.LittleEndian.PutUint32(b[:], x)
			inst, err := riscv64asm.Decode(b[:])
			if err != nil {
				continue
			}
			// Expected record built from the reference's decoded operands.
			want, ok := expected(inst, x, c)
			if !ok {
				continue
			}
			r.checked++
			// 1) compute_op must match (covers wrong mapping and wrong x0 folding).
			if got.op != want.op {
				r.opMis++
				note("op:"+inst.Op.String(), fmt.Sprintf("%#08x %v got=%d want=%d", x, inst, got.op, want.op))
				continue
			}
			// NO_OP (0) carries no meaningful operands, so don't compare them.
			if want.op == 0 {
				continue
			}
			// 2) The whole record (imm, rs1, rs2, rd) must match.
			if got != want {
				r.operandMis++
				note("operands:"+inst.Op.String(), fmt.Sprintf("%#08x %v got=%+v want=%+v", x, inst, got, want))
			}
		}
		return r
	})

	// Merge per-worker results.
	var checked, opMis, operandMis, custom uint64
	cats := map[string]uint64{}
	samples := map[string]string{}
	for _, r := range results {
		checked += r.checked
		opMis += r.opMis
		operandMis += r.operandMis
		custom += r.custom
		for k, v := range r.cats {
			cats[k] += v
			if _, ok := samples[k]; !ok {
				samples[k] = r.samples[k]
			}
		}
	}
	// Report each mismatch category in sorted order, then fail if there were any.
	keys := make([]string, 0, len(cats))
	for k := range cats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("MISMATCH %-22s count=%d e.g. %s", k, cats[k], samples[k])
	}
	t.Logf("summary: RV64IM words checked=%d op-mismatch=%d operand-mismatch=%d custom1-words=%d", checked, opMis, operandMis, custom)
	if len(cats) > 0 {
		t.Errorf("%d mismatch categories", len(cats))
	}
}

// diffRes is one worker's partial result for TestDiffRefRVOP; results are
// merged after all workers finish, so no locking is needed while scanning.
type diffRes struct {
	// mismatch counts words per mismatch category (see the key names below).
	mismatch map[string]uint64
	// samples keeps the first word seen per mismatch category, for reproduction.
	samples map[string]uint32
	// opMap records, for words both sides accept, which reference mnemonics
	// were seen for each Lineth compute_op.
	opMap map[uint32]map[string]bool
}

// Compares Lineth's accept/reject verdict (classifyInstruction) with the
// reference on all 2^32 words. "Valid RV64IM" on the reference side means the
// reference decodes the word AND its mnemonic is in the spec table above — the
// reference understands far more (A, F, D, C, Zicsr, ...). Words Lineth accepts
// but the reference rejects are grouped by (opcode, funct3, compute_op).
// It only logs; it never fails the test, so read the -v output for mismatches.
func TestDiffRefRVOP(t *testing.T) {
	results := forEachWord(func(lo, hi uint64) diffRes {
		r := diffRes{map[string]uint64{}, map[string]uint32{}, map[uint32]map[string]bool{}}
		var b [4]byte
		for i := lo; i < hi; i++ {
			x := uint32(i)
			// Lineth's verdict: anything other than computeInvalid is "accepted".
			l := classifyInstruction(x)
			lv := l != computeInvalid
			// key names the mismatch category for this word; empty means no mismatch.
			key := ""
			if x&3 != 3 {
				// Low two bits != 0b11 means a 16-bit compressed (RVC) encoding.
				// Lineth has no C extension, so it must reject all of these.
				if lv {
					key = "lineth-valid-on-16bit-encoding"
				}
			} else if x&0x7f == opcodeCUSTOM1 {
				// Lineth-specific custom opcode; the reference decoder knows nothing
				// about it (TestRecordsAgainstSpec checks it against its own spec).
				continue
			} else {
				// Decode with the reference (riscv64asm expects little-endian bytes).
				binary.LittleEndian.PutUint32(b[:], x)
				inst, err := riscv64asm.Decode(b[:])
				name := ""
				if err == nil {
					name = inst.Op.String()
				}
				// Reference verdict: decodes successfully AND is an RV64IM mnemonic.
				_, inSpec := spec[name]
				rv := err == nil && inSpec
				switch {
				case lv && !rv:
					// Lineth accepts a word the spec says is not RV64IM. Grouped by
					// opcode/funct3/compute_op so the extras can be told apart.
					if name == "" {
						name = "(not an instruction)"
					}
					key = fmt.Sprintf("lineth-valid/ref-not-RV64IM: %s opcode=%07b funct3=%03b compute_op=%d", name, x&0x7f, (x>>12)&7, l)
				case !lv && rv:
					// Lineth rejects a real RV64IM instruction (completeness gap).
					key = "lineth-invalid/ref-RV64IM: " + name
				case lv && rv && l != computeNoOp:
					// Both accept: record which reference mnemonic landed in which
					// compute_op, to check the mapping is consistent below.
					// computeNoOp is excluded because it intentionally groups many ops.
					m := r.opMap[l]
					if m == nil {
						m = map[string]bool{}
						r.opMap[l] = m
					}
					m[name] = true
				}
			}
			if key != "" {
				r.mismatch[key]++
				if _, ok := r.samples[key]; !ok {
					r.samples[key] = x
				}
			}
		}
		return r
	})

	// Merge per-worker results.
	mis := map[string]uint64{}
	smp := map[string]uint32{}
	ops := map[uint32]map[string]bool{}
	for _, r := range results {
		for k, v := range r.mismatch {
			mis[k] += v
			if _, ok := smp[k]; !ok {
				smp[k] = r.samples[k]
			}
		}
		for k, m := range r.opMap {
			if ops[k] == nil {
				ops[k] = map[string]bool{}
			}
			for n := range m {
				ops[k][n] = true
			}
		}
	}
	// Report mismatch categories in sorted order, with count and one sample word.
	keys := make([]string, 0, len(mis))
	for k := range mis {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var overAccepted uint64
	for _, k := range keys {
		t.Logf("MISMATCH %-90s count=%d sample=%#08x", k, mis[k], smp[k])
		if len(k) > 27 && k[:27] == "lineth-valid/ref-not-RV64IM" {
			overAccepted += mis[k]
		}
	}
	// Mapping consistency, forward: one compute_op covering several reference ops.
	// Also builds the reverse index (reference op -> compute_ops) for the check below.
	multi := 0
	rev := map[string][]uint32{}
	for op, names := range ops {
		if len(names) > 1 {
			multi++
			t.Logf("compute_op %d maps to several ref ops: %v", op, names)
		}
		for n := range names {
			rev[n] = append(rev[n], op)
		}
	}
	// Mapping consistency, reverse: one reference op split across several compute_ops.
	// Expected only for JAL/JALR (plain vs _WB, chosen by rd == x0).
	for n, l := range rev {
		if len(l) > 1 {
			t.Logf("ref op %s maps to several compute_ops: %v", n, l)
		}
	}
	t.Logf("summary: mismatch-categories=%d over-accepted=%d compute_ops-seen=%d ambiguous=%d", len(mis), overAccepted, len(ops), multi)
}
