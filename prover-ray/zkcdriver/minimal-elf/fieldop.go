package minimalelf

import (
	"encoding/binary"
	"math/big"
)

// Field accelerator ECALL (arithmetization/src/main/lib/field/field.zkc):
// a7 = fieldOpSyscall, a0 = op, a1 = field, a2/a3/a4 = out/a/b pointers to four
// little-endian u64 limbs; the status comes back in a0.
const (
	fieldOpSyscall = 0xF0

	fieldOpMul  = 0
	fieldOpDiv  = 1
	fieldOpSqrt = 2

	fieldSecp256k1Fp = 1
	fieldSecp256k1Fr = 2
	fieldBN254Fr     = 6
	fieldBLS12381Fr  = 7
)

var fieldModuli = map[int]string{
	fieldSecp256k1Fp: "fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f",
	fieldSecp256k1Fr: "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141",
	fieldBN254Fr:     "30644e72e131a029b85045b68181585d2833e84879b9709143e1f593f0000001",
	fieldBLS12381Fr:  "73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001",
}

type fieldOpCase struct {
	op, field int
	a, b      *big.Int
}

// FieldOpELF returns a minimal valid ELF64 RISC-V binary that issues a few
// field accelerator calls (MUL, DIV, inverse as DIV(1, x), SQRT of a square and
// of a non-square, x/0, an unreduced input, a small quotient) and writes each 32-byte result and
// its u64 status to guest_output, together with that expected output.
func FieldOpELF() (elf, output []byte) {
	p := func(field int) *big.Int { v, _ := new(big.Int).SetString(fieldModuli[field], 16); return v }
	hex := func(s string) *big.Int { v, _ := new(big.Int).SetString(s, 16); return v }
	x := hex("0123456789abcdeffedcba98765432100f1e2d3c4b5a69788796a5b4c3d2e1f0")
	y := hex("5555555555555555555555555555555555555555555555555555555555555555")
	pk := p(fieldSecp256k1Fp)
	sq := new(big.Int).Mul(x, x)
	sq.Mod(sq, pk)
	cases := []fieldOpCase{
		{fieldOpMul, fieldSecp256k1Fp, x, y},
		{fieldOpDiv, fieldSecp256k1Fp, x, y},
		{fieldOpDiv, fieldSecp256k1Fr, big.NewInt(1), x},
		{fieldOpSqrt, fieldSecp256k1Fp, sq, sq},
		{fieldOpSqrt, fieldSecp256k1Fp, new(big.Int).Sub(pk, sq), nil},
		{fieldOpDiv, fieldBN254Fr, big.NewInt(7), big.NewInt(0)},
		{fieldOpMul, fieldSecp256k1Fp, new(big.Int).Add(big.NewInt(5), pk), y},
		{fieldOpMul, fieldBLS12381Fr, new(big.Int).Mod(x, p(fieldBLS12381Fr)), new(big.Int).Mod(y, p(fieldBLS12381Fr))},
		// a small quotient (5), whose unreduced form 5 + p still fits in 256 bits
		{fieldOpDiv, fieldSecp256k1Fp, new(big.Int).Mod(new(big.Int).Mul(big.NewInt(5), y), pk), y},
	}

	// Layout, from the auipc base: code, then the operands (64 bytes per case),
	// then the outputs (40 bytes per case) in zeroed memory past the section.
	const codeWords = 8 + 10*8 + 8 // generous upper bound, padded below
	const dataOff = codeWords * 4
	outOff := dataOff + 64*len(cases)

	a := newAssembler()
	a.auipc(s0, 0)
	for i, c := range cases {
		a.addi(a0, zero, int32(c.op))
		a.addi(a1, zero, int32(c.field))
		a.addi(a2, s0, int32(outOff+40*i))
		a.addi(a3, s0, int32(dataOff+64*i))
		a.addi(a4, s0, int32(dataOff+64*i+32))
		a.addi(a7, zero, fieldOpSyscall)
		a.ecall()
		a.sd(a0, s0, int32(outOff+40*i+32))
	}
	a.addi(t1, s0, int32(outOff))
	a.addi(t3, zero, int32(40*len(cases)))
	a.writeOutput(t1, t3)
	a.addi(a0, zero, 0)
	a.addi(a7, zero, 93) // exit(0)
	a.ecall()
	if len(a.words) > codeWords {
		panic("minimalelf: FieldOpELF code overflows its data offset")
	}
	for len(a.words) < codeWords {
		a.addi(zero, zero, 0) // nop
	}
	for _, c := range cases {
		b := c.b
		if b == nil {
			b = c.a
		}
		for _, v := range []*big.Int{c.a, b} {
			limbs := le256(v)
			for i := 0; i < 32; i += 4 {
				a.emit(binary.LittleEndian.Uint32(limbs[i:]))
			}
		}
	}

	for _, c := range cases {
		res, status := fieldOpReference(c, p(c.field))
		output = append(output, le256(res)...)
		output = binary.LittleEndian.AppendUint64(output, status)
	}
	return Make(DefaultEntryPoint, DefaultSectionAddr, a.bytes()), output
}

// fieldOpReference is what an honest prover returns for c.
func fieldOpReference(c fieldOpCase, p *big.Int) (*big.Int, uint64) {
	a := new(big.Int).Mod(c.a, p)
	switch c.op {
	case fieldOpMul:
		r := new(big.Int).Mul(a, new(big.Int).Mod(c.b, p))
		return r.Mod(r, p), 0
	case fieldOpDiv:
		b := new(big.Int).Mod(c.b, p)
		if b.Sign() == 0 {
			return new(big.Int), 1
		}
		r := new(big.Int).ModInverse(b, p)
		r.Mul(r, a)
		return r.Mod(r, p), 0
	default: // sqrt, p = 3 mod 4: a^((p+1)/4)
		e := new(big.Int).Add(p, big.NewInt(1))
		e.Rsh(e, 2)
		r := new(big.Int).Exp(a, e, p)
		sq := new(big.Int).Mul(r, r)
		if sq.Mod(sq, p).Cmp(a) == 0 {
			return r, 0
		}
		return r, 1
	}
}

func le256(v *big.Int) []byte {
	be := v.FillBytes(make([]byte, 32))
	for i, j := 0, 31; i < j; i, j = i+1, j-1 {
		be[i], be[j] = be[j], be[i]
	}
	return be
}
