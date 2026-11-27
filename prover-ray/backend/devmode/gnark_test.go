package devmode_test

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/stretchr/testify/require"
)

// gnarkPub returns devmode public inputs with full guest-output bytes (0xff
// first, so the packed value exceeds r and is reduced) and program-VK limbs
// near the KoalaBear modulus (so a Montgomery form would differ).
func gnarkPub() wiop.PublicInput {
	pub := make(wiop.PublicInput, devmode.NumPublicInputs)
	for i := range devmode.NumGuestOutputCells {
		pub[i] = base(uint64(0xff - i))
	}
	for i := range devmode.NumProgramVKCells {
		// 0x7f000000 = p-1 (KoalaBear): the largest canonical value, with a nonzero top byte; -i fills the low bytes.
		// Elements are stored as x·2^32 mod p (Montgomery); reading e[0] instead of e.Uint64() would pack the wrong bytes.
		pub[devmode.NumGuestOutputCells+i] = base(uint64(0x7f000000 - i))
	}
	return pub
}

func asBase(pub wiop.PublicInput) []field.Element {
	out := make([]field.Element, len(pub))
	for i, x := range pub {
		out[i] = x.AsBase()
	}
	return out
}

// TestPack checks the packing against the byte layout the contract hashes.
func TestPack(t *testing.T) {
	r := ecc.BN254.ScalarField()

	out := make([]field.Element, devmode.NumGuestOutputCells)
	var outBytes [32]byte
	for i := range out {
		outBytes[i] = byte(0xff - i)
		out[i].SetUint64(uint64(outBytes[i]))
	}
	want := new(big.Int).SetBytes(outBytes[:])
	require.Positive(t, want.Cmp(r), "the test value must exceed r to exercise the reduction")
	require.Equal(t, want.Mod(want, r), devmode.PackGuestOutput(out))

	vk := make([]field.Element, devmode.NumProgramVKCells)
	var vkBytes [32]byte
	for i := range vk {
		limb := uint32(0x7f000000 - i)
		vk[i].SetUint64(uint64(limb))
		binary.BigEndian.PutUint32(vkBytes[4*i:], limb)
	}
	want = new(big.Int).SetBytes(vkBytes[:])
	require.Equal(t, want.Mod(want, r), devmode.PackProgramVK(vk))
}

// TestGnarkCircuit_PLONKProve verifies the proof of [devmode.GnarkProve] on
// public inputs from [devmode.Prove] under the key of a
// separate [devmode.GnarkSetup], so the keys are the same on every setup. It
// also checks that the proof is rejected under a changed public input.
func TestGnarkCircuit_PLONKProve(t *testing.T) {
	modulus := ecc.BN254.ScalarField()

	sys, err := devmode.NewSystem()
	require.NoError(t, err)
	_, pub, err := devmode.Prove(sys, gnarkPub())
	require.NoError(t, err)

	ccs, pk, vk, err := devmode.GnarkSetup()
	require.NoError(t, err)

	proof, err := devmode.GnarkProve(sys, pub)
	require.NoError(t, err)

	public, err := frontend.NewWitness(devmode.NewGnarkAssignment(sys, pub), modulus, frontend.PublicOnly())
	require.NoError(t, err)
	t.Logf("constraints=%d", ccs.GetNbConstraints())
	require.NoError(t, plonk.Verify(proof, vk, public), "PLONK proof must verify")

	// The public witness is exactly the two packed values, guest output first.
	vec, ok := public.Vector().(fr.Vector)
	require.True(t, ok)
	require.Len(t, vec, 2)
	var wantOut, wantVK fr.Element
	wantOut.SetBigInt(devmode.PackGuestOutput(asBase(pub[:devmode.NumGuestOutputCells])))
	wantVK.SetBigInt(devmode.PackProgramVK(asBase(pub[devmode.NumGuestOutputCells:])))
	require.Equal(t, wantOut, vec[0], "public input 0 is the packed guest output")
	require.Equal(t, wantVK, vec[1], "public input 1 is the packed program VK")

	// The same proof is rejected under a changed public input.
	for i := range vec {
		changed := &devmode.GnarkCircuit{GuestOutput: vec[0], ProgramVK: vec[1]}
		if i == 0 {
			changed.GuestOutput = new(big.Int).Add(devmode.PackGuestOutput(asBase(pub[:devmode.NumGuestOutputCells])), big.NewInt(1))
		} else {
			changed.ProgramVK = new(big.Int).Add(devmode.PackProgramVK(asBase(pub[devmode.NumGuestOutputCells:])), big.NewInt(1))
		}
		badPublic, err := frontend.NewWitness(changed, modulus, frontend.PublicOnly())
		require.NoError(t, err)
		require.ErrorContains(t, plonk.Verify(proof, vk, badPublic), "algebraic relation does not hold",
			"changed public input %d must be rejected", i)
	}

	// A wrong packed value cannot be proven.
	bad := devmode.NewGnarkAssignment(sys, pub)
	bad.GuestOutputCells[0] = 0
	badW, err := frontend.NewWitness(bad, modulus)
	require.NoError(t, err)
	_, err = plonk.Prove(ccs, pk, badW)
	require.ErrorContains(t, err, "is not satisfied")
}
