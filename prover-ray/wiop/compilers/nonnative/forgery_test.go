package nonnative_test

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullPipeline_RejectsResultOffByFieldModulus replays the forgery that
// breaks a digit layout whose coefficients can wrap modulo the field modulus
// p: on row 0 the prover claims Result + p, re-solves the carries over the
// field so that the polynomial identity still vanishes coefficient-wise, and
// keeps every other column honest. The forged witness satisfies the identity
// (asserted below), so only the digit and carry range checks can reject it.
//
// With 16-bit digits the forged carries stay below 2^18 and Result + p has
// valid 16-bit limbs, so such a witness used to be accepted.
func TestFullPipeline_RejectsResultOffByFieldModulus(t *testing.T) {
	for _, build := range []func() *wioptest.NonNativeScenario{
		wioptest.NewNonNativeU64Scenario,
		wioptest.NewNonNativeU128Scenario,
		wioptest.NewNonNativeU256Scenario,
	} {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			q := sc.Sys.Modules[0].NonNatives[0]
			forge := &forgeResultPlusModulus{query: q}
			hook := func(*wiop.System) error {
				forge.carry = compiledColumns(q.Module, "/carry/limb-")
				// Limbs are their own digits when the compiler does not split them.
				if forge.resultDigits = compiledColumns(q.Module, "/result/digit-"); len(forge.resultDigits) == 0 {
					forge.resultDigits = q.Result
				}
				q.InputRound.RegisterAction(forge)
				return nil
			}
			require.NoError(t, compilers.CompileFull(sc.Sys, append(
				wioptest.TestingCompileOptions(),
				compilers.WithPostHook(compilers.NonNative, hook),
			)...))
			require.NotEmpty(t, forge.resultDigits, "result digit columns not found")
			require.NotEmpty(t, forge.carry, "carry columns not found")

			err := proveAndVerify(sc.Sys, sc.AssignHonest)
			require.True(t, forge.forged, "Result + p must fit the result limbs")
			require.True(t, forge.identityHolds, "the forged witness must satisfy the identity modulo p")
			assert.Error(t, err, "full pipeline must reject Result + p")
		})
	}
}

// proveAndVerify runs the prover and verifier, reporting a prover panic as a
// rejection: the run produced no accepting transcript.
func proveAndVerify(sys *wiop.System, assign func(*wiop.Runtime)) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("prover panicked: %v", r)
		}
	}()
	proof, pub := sys.Prove(assign)
	return sys.Verify(proof, pub)
}

// compiledColumns returns the columns of m whose path contains marker
// followed by an index, ordered by that index.
func compiledColumns(m *wiop.Module, marker string) []*wiop.Column {
	var cols []*wiop.Column
	for _, c := range m.Columns {
		_, idx, found := strings.Cut(c.Context.Path(), marker)
		if !found {
			continue
		}
		i, err := strconv.Atoi(idx)
		if err != nil {
			panic(err)
		}
		cols = append(cols, make([]*wiop.Column, max(i+1-len(cols), 0))...)
		cols[i] = c
	}
	return cols
}

// forgeResultPlusModulus runs after the compiler's own prover action and
// rewrites row 0 to claim Result + p.
type forgeResultPlusModulus struct {
	query        *wiop.NonNative
	resultDigits []*wiop.Column
	carry        []*wiop.Column

	forged        bool
	identityHolds bool
}

func (f *forgeResultPlusModulus) Run(rt *wiop.Runtime) {
	const row = 0
	q := f.query
	limbBits := uint(q.NbBitsPerLimb)
	digitsPerLimb := len(f.resultDigits) / len(q.Result)
	digitBits := limbBits / uint(digitsPerLimb)

	oldDigits := readRow(rt, f.resultDigits, row)
	forged := composeRow(rt, q.Result, row, limbBits)
	forged.Add(forged, field.Modulus())
	if forged.BitLen() > len(q.Result)*int(limbBits) {
		return
	}
	newLimbs := splitBits(forged, len(q.Result), limbBits)
	newDigits := splitBits(forged, len(f.resultDigits), digitBits)

	// The identity is linear in result and carry: claiming result + Δ keeps it
	// vanishing iff the carries move by Δc with Δc_k = (Δc_{k-1} - Δres_k) / 2^s.
	var radix, radixInv field.Element
	radix.SetUint64(uint64(1) << digitBits)
	radixInv.Inverse(&radix)
	resDelta := func(k int) field.Element {
		var d field.Element
		if k < len(newDigits) {
			d.Sub(&newDigits[k], &oldDigits[k])
		}
		return d
	}
	carries := readRow(rt, f.carry, row)
	var carryDelta field.Element
	for k := range carries {
		d := resDelta(k)
		carryDelta.Sub(&carryDelta, &d)
		carryDelta.Mul(&carryDelta, &radixInv)
		carries[k].Add(&carries[k], &carryDelta)
	}
	// Leading coefficient: Δc_{last} - Δres_{last+1} must vanish.
	top := resDelta(len(carries))
	top.Sub(&carryDelta, &top)
	f.identityHolds = top.IsZero()

	overrideRow(rt, q.Result, row, newLimbs)
	overrideRow(rt, f.resultDigits, row, newDigits)
	overrideRow(rt, f.carry, row, carries)
	f.forged = true
}

// composeRow recomposes the little-endian limbs of one row.
func composeRow(rt *wiop.Runtime, limbs []*wiop.Column, row int, limbBits uint) *big.Int {
	v := new(big.Int)
	for i := len(limbs) - 1; i >= 0; i-- {
		limb := rowValue(rt, limbs[i], row)
		v.Lsh(v, limbBits)
		v.Add(v, new(big.Int).SetUint64(limb.Uint64()))
	}
	return v
}

func splitBits(v *big.Int, n int, width uint) []field.Element {
	out := make([]field.Element, n)
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), width), big.NewInt(1))
	rem := new(big.Int).Set(v)
	for i := range out {
		out[i].SetBigInt(new(big.Int).And(rem, mask))
		rem.Rsh(rem, width)
	}
	return out
}

func rowValue(rt *wiop.Runtime, col *wiop.Column, row int) field.Element {
	return rt.GetColumnAssignment(col).Plain.AsBase()[row]
}

func readRow(rt *wiop.Runtime, cols []*wiop.Column, row int) []field.Element {
	out := make([]field.Element, len(cols))
	for i, c := range cols {
		out[i] = rowValue(rt, c, row)
	}
	return out
}

func overrideRow(rt *wiop.Runtime, cols []*wiop.Column, row int, vals []field.Element) {
	for i, c := range cols {
		cv := rt.GetColumnAssignment(c)
		plain := append([]field.Element(nil), cv.Plain.AsBase()...)
		plain[row] = vals[i]
		rt.OverrideColumn(c, &wiop.ConcreteVector{Plain: field.VecFromBase(plain), Padding: cv.Padding})
	}
}
