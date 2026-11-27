package wiop_test

import (
	"encoding/binary"
	"maps"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/plonk"
	cs_bn254 "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/test/unsafekzg"
	"github.com/stretchr/testify/require"
)

// wrapperPub returns devmode public inputs whose guest output packs to a value
// above r (so the reduction runs) and whose program-VK limbs are near p.
func wrapperPub() wiop.PublicInput {
	pub := make(wiop.PublicInput, devmode.NumPublicInputs)
	for i := range pub {
		v := uint64(0xff - i)
		if i >= devmode.NumGuestOutputCells {
			v = uint64(0x7f000000 - i)
		}
		var e field.Element
		e.SetUint64(v)
		pub[i] = field.ElemFromBase(e)
	}
	return pub
}

// TestPackBigEndian checks the packing against the byte layout the contract
// hashes: bytes big-endian, VK limb i in bytes 4i to 4i+4, reduced mod r.
func TestPackBigEndian(t *testing.T) {
	r := ecc.BN254.ScalarField()

	var outBytes, vkBytes [32]byte
	out := make([]field.Element, 32)
	for i := range out {
		outBytes[i] = byte(0xff - i)
		out[i].SetUint64(uint64(outBytes[i]))
	}
	vk := make([]field.Element, 8)
	for i := range vk {
		limb := uint32(0x7f000000 - i)
		vk[i].SetUint64(uint64(limb))
		binary.BigEndian.PutUint32(vkBytes[4*i:], limb)
	}

	want := new(big.Int).SetBytes(outBytes[:])
	require.Positive(t, want.Cmp(r), "the test value must exceed r to exercise the reduction")
	require.Equal(t, want.Mod(want, r), wiop.PackBigEndian(out, wiop.GuestOutputBitsPerCell))
	want = new(big.Int).SetBytes(vkBytes[:])
	require.Equal(t, want.Mod(want, r), wiop.PackBigEndian(vk, wiop.ProgramVKBitsPerCell))
}

// TestWrapperCircuit_Misuse checks that allocation rejects a missing group.
func TestWrapperCircuit_Misuse(t *testing.T) {
	sys := newWrapperSystem(t)
	require.PanicsWithValue(t,
		"wiop: WrapperCircuit: 0 public inputs tagged Nope of 8 bits each, want 256 bits",
		func() { wiop.AllocateWrapperCircuit(sys, "Nope", risc5.ProgramVKPI) },
		"a missing group must be rejected")
	require.PanicsWithValue(t,
		"wiop: WrapperCircuit: 8 public inputs tagged ProgramVerificationKey of 8 bits each, want 256 bits",
		func() { wiop.AllocateWrapperCircuit(sys, risc5.ProgramVKPI, risc5.GuestPublicOutputsPI) },
		"swapped tags have the wrong number of cells for their width")
}

// TestWrapperCircuit_PLONKProve wraps a devmode wiop proof: PLONK prove and
// verify over BN254 and the proof is rejected under a changed public input.
// The checks that need no proof are in [TestWrapperCircuit_IsSolved]. The SRS
// comes from [unsafekzg].
func TestWrapperCircuit_PLONKProve(t *testing.T) {
	if testing.Short() {
		t.Skip("PLONK setup and proving")
	}
	modulus := ecc.BN254.ScalarField()

	sys := newWrapperSystem(t)
	proof, pub, err := devmode.Prove(sys, wrapperPub())
	require.NoError(t, err)
	require.NoError(t, devmode.Verify(sys, proof, pub))

	circ := wiop.AllocateWrapperCircuit(sys, risc5.GuestPublicOutputsPI, risc5.ProgramVKPI)
	ccs, err := frontend.Compile(modulus, scs.NewBuilder, circ)
	require.NoError(t, err)
	srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
	require.NoError(t, err)
	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	require.NoError(t, err)

	w, err := frontend.NewWitness(circ.AssignWrapperCircuit(proof, pub), modulus)
	require.NoError(t, err)
	public, err := w.Public()
	require.NoError(t, err)

	start := time.Now()
	plonkProof, err := plonk.Prove(ccs, pk, w)
	require.NoError(t, err, "an honest wiop proof must be provable")
	t.Logf("constraints=%d prove=%v", ccs.GetNbConstraints(), time.Since(start).Round(time.Millisecond))
	require.NoError(t, plonk.Verify(plonkProof, vk, public), "PLONK proof must verify")

	// The same proof is rejected under a changed public input.
	wantOut, wantVK := wantPacked(sys, pub)
	one := big.NewInt(1)
	for i, changed := range []*wiop.WrapperCircuit{
		{GuestOutput: new(big.Int).Add(wantOut, one), ProgramVK: wantVK},
		{GuestOutput: wantOut, ProgramVK: new(big.Int).Add(wantVK, one)},
	} {
		badPublic, err := frontend.NewWitness(changed, modulus, frontend.PublicOnly())
		require.NoError(t, err)
		require.ErrorContains(t, plonk.Verify(plonkProof, vk, badPublic), "algebraic relation does not hold",
			"changed public input %d must be rejected", i)
	}
}

// TestWrapperCircuit_IsSolved checks the wrapper without PLONK setup, by
// solving the circuit: the honest witness solves and its public part is the two
// packed values, guest output first; a tampered wiop proof does not solve; and
// a non-canonical public-input cell does not solve.
func TestWrapperCircuit_IsSolved(t *testing.T) {
	modulus := ecc.BN254.ScalarField()

	sys := newWrapperSystem(t)
	proof, pub, err := devmode.Prove(sys, wrapperPub())
	require.NoError(t, err)

	circ := wiop.AllocateWrapperCircuit(sys, risc5.GuestPublicOutputsPI, risc5.ProgramVKPI)
	ccs, err := frontend.Compile(modulus, scs.NewBuilder, circ)
	require.NoError(t, err)

	honest, err := frontend.NewWitness(circ.AssignWrapperCircuit(proof, pub), modulus)
	require.NoError(t, err)
	require.NoError(t, ccs.IsSolved(honest), "the honest witness must solve")

	t.Run("public-witness", func(t *testing.T) {
		// The public witness is exactly the two packed values, guest output
		// first, and agrees with the devmode gnark circuit.
		public, err := honest.Public()
		require.NoError(t, err)
		wantOut, wantVK := wantPacked(sys, pub)
		vec, ok := public.Vector().(fr.Vector)
		require.True(t, ok, "the public witness must be a BN254 vector")
		require.Len(t, vec, 2, "the public witness must be the two packed values")
		var e0, e1 fr.Element
		e0.SetBigInt(wantOut)
		e1.SetBigInt(wantVK)
		require.Equal(t, e0, vec[0], "public input 0 is the packed guest output")
		require.Equal(t, e1, vec[1], "public input 1 is the packed program VK")
	})

	t.Run("tampered-proof", func(t *testing.T) {
		// Tamper with the commitment of the first committed round. Picking the
		// smallest ID keeps the test deterministic: map order is random.
		bad := wiop.Proof{Cells: proof.Cells, Commitments: maps.Clone(proof.Commitments), PCSOpeningProof: proof.PCSOpeningProof}
		require.NotEmpty(t, bad.Commitments)
		id := slices.Min(slices.Collect(maps.Keys(bad.Commitments)))
		c := bad.Commitments[id]
		c[0].Add(&c[0], new(field.Element).SetOne())
		bad.Commitments[id] = c
		badW, err := frontend.NewWitness(circ.AssignWrapperCircuit(bad, pub), modulus)
		require.NoError(t, err)
		requireUnsatisfied(t, ccs.IsSolved(badW), "a tampered wiop proof must not solve")
	})

	t.Run("non-canonical-cell", func(t *testing.T) {
		// A public-input cell assigned x+p instead of x cannot be packed: the
		// wiop verifier only sees cells mod p, so without the strict reduction
		// this witness would solve and expose a packed guest output the
		// contract would never compute.

		// Guest-output byte 0 is wiop public input 0 and, as devmode declares its
		// public-input cells first and in order, PublicInputsBase[0]. It is a byte,
		// so x+p still fits the 31-bit emulated limb.
		_, pos := sys.LookupPublicInputByTag(risc5.GuestPublicOutputsPI, 0)
		require.Equal(t, 0, pos, "guest-output byte 0 must be wiop public input 0")
		p := new(big.Int).SetUint64(field.Modulus().Uint64())
		x := new(big.Int).SetUint64(pub[0].AsBase().Uint64())
		nonCanonical := new(big.Int).Add(x, p)

		bad := circ.AssignWrapperCircuit(proof, pub)
		bad.Verifier.PublicInputsBase[0] = circuit.Element{EV: emulated.Element[emulated.KoalaBear]{
			Limbs: []frontend.Variable{nonCanonical},
		}}
		// The packed value consistent with x+p: byte 0 has weight 2^(8*31).
		shift := new(big.Int).Lsh(p, 8*31)
		bad.GuestOutput = new(big.Int).Mod(new(big.Int).Add(bad.GuestOutput.(*big.Int), shift), modulus)

		badW, err := frontend.NewWitness(bad, modulus)
		require.NoError(t, err)
		requireUnsatisfied(t, ccs.IsSolved(badW), "a non-canonical public-input cell must be rejected")
	})
}

// wantPacked returns the packed guest output and program VK of pub, computed
// by devmode rather than by the wrapper.
func wantPacked(sys *wiop.System, pub wiop.PublicInput) (out, vk *big.Int) {
	programVK := risc5.GetProgramVK(sys, pub)
	return devmode.PackGuestOutput(risc5.GetGuestPublicOutputs(sys, pub)), devmode.PackProgramVK(programVK[:])
}

// requireUnsatisfied requires err to be the solver rejecting a constraint, not
// any other failure (a bad assignment, a missing value).
func requireUnsatisfied(t *testing.T, err error, msg string) {
	t.Helper()
	var unsat *cs_bn254.UnsatisfiedConstraintError
	require.ErrorAs(t, err, &unsat, msg)
}

// TestWrapperCircuit_GuestOutputNotByte checks that a guest-output cell wider
// than a byte is rejected. The wiop proof is valid, since devmode does not
// constrain the guest output to bytes, so only the wrapper's 8-bit range check
// can reject it.
func TestWrapperCircuit_GuestOutputNotByte(t *testing.T) {
	modulus := ecc.BN254.ScalarField()

	sys := newWrapperSystem(t)
	in := wrapperPub()
	_, pos := sys.LookupPublicInputByTag(risc5.GuestPublicOutputsPI, 0)
	in[pos] = field.ElemFromBase(field.NewElement(300))
	proof, pub, err := devmode.Prove(sys, in)
	require.NoError(t, err)
	require.NoError(t, devmode.Verify(sys, proof, pub), "the wiop proof itself must be valid")

	circ := wiop.AllocateWrapperCircuit(sys, risc5.GuestPublicOutputsPI, risc5.ProgramVKPI)
	ccs, err := frontend.Compile(modulus, scs.NewBuilder, circ)
	require.NoError(t, err)
	w, err := frontend.NewWitness(circ.AssignWrapperCircuit(proof, pub), modulus)
	require.NoError(t, err)
	// gnark's lookup-based range check reports an out-of-range value from its
	// solving hint, not as an unsatisfied constraint.
	require.ErrorContains(t, ccs.IsSolved(w), "query element not in table",
		"a guest-output cell of 300 must be rejected by the range check")
}

// newWrapperSystem builds the devmode system without column elision, which
// the in-circuit verifier does not support.
func newWrapperSystem(t *testing.T) *wiop.System {
	t.Helper()
	sys, err := devmode.NewSystem(compilers.ConfigurePCS(
		pcs.WithFRINumQueries(devmode.FRINumQueries),
		pcs.WithOptimalMerkleCap(),
	))
	require.NoError(t, err)
	return sys
}
