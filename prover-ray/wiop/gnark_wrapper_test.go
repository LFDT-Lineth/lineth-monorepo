package wiop_test

import (
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
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
	require.Panics(t, func() { wiop.AllocateWrapperCircuit(sys, "Nope", risc5.ProgramVKPI) })
	require.Panics(t, func() { wiop.AllocateWrapperCircuit(sys, risc5.ProgramVKPI, risc5.GuestPublicOutputsPI) },
		"swapped tags have the wrong number of cells for their width")
}

// TestWrapperCircuit_PLONKProve wraps a devmode wiop proof: PLONK prove and
// verify over BN254, the public witness is the two packed values, the proof is
// rejected under a changed public input, and a tampered wiop proof cannot be
// proven. The SRS comes from [unsafekzg].
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

	// The public witness is exactly the two packed values, guest output first,
	// and agrees with the devmode gnark circuit.
	wantOut := devmode.PackGuestOutput(risc5.GetGuestPublicOutputs(sys, pub))
	programVK := risc5.GetProgramVK(sys, pub)
	wantVK := devmode.PackProgramVK(programVK[:])
	vec, ok := public.Vector().(fr.Vector)
	require.True(t, ok)
	require.Len(t, vec, 2)
	var e0, e1 fr.Element
	e0.SetBigInt(wantOut)
	e1.SetBigInt(wantVK)
	require.Equal(t, e0, vec[0], "public input 0 is the packed guest output")
	require.Equal(t, e1, vec[1], "public input 1 is the packed program VK")

	// The same proof is rejected under a changed public input.
	one := big.NewInt(1)
	for i, changed := range []*wiop.WrapperCircuit{
		{GuestOutput: new(big.Int).Add(wantOut, one), ProgramVK: wantVK},
		{GuestOutput: wantOut, ProgramVK: new(big.Int).Add(wantVK, one)},
	} {
		badPublic, err := frontend.NewWitness(changed, modulus, frontend.PublicOnly())
		require.NoError(t, err)
		require.Error(t, plonk.Verify(plonkProof, vk, badPublic), "changed public input %d must be rejected", i)
	}

	// A tampered wiop proof cannot be proven.
	bad := wiop.Proof{Cells: proof.Cells, Commitments: map[int]field.Octuplet{}, PCSOpeningProof: proof.PCSOpeningProof}
	for id, c := range proof.Commitments {
		bad.Commitments[id] = c
	}
	for id, c := range bad.Commitments {
		c[0].Add(&c[0], new(field.Element).SetOne())
		bad.Commitments[id] = c
		break
	}
	badW, err := frontend.NewWitness(circ.AssignWrapperCircuit(bad, pub), modulus)
	require.NoError(t, err)
	_, err = plonk.Prove(ccs, pk, badW)
	require.ErrorContains(t, err, "is not satisfied", "a tampered wiop proof must not be provable")
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
