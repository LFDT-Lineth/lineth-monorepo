package devmode_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

func base(v uint64) field.Gen {
	var e field.Element
	e.SetUint64(v)
	return field.ElemFromBase(e)
}

func samplePub() wiop.PublicInput {
	pub := make(wiop.PublicInput, devmode.NumPublicInputs)
	for i := range pub {
		pub[i] = base(uint64(1000 + i))
	}
	return pub
}

func TestProveVerify(t *testing.T) {
	sys, err := devmode.NewSystem()
	require.NoError(t, err)

	proof, pub, err := devmode.Prove(sys, samplePub())
	require.NoError(t, err)
	require.Equal(t, samplePub(), pub, "the statement is the public inputs given to Prove")
	require.NoError(t, devmode.Verify(sys, proof, pub), "an honest proof must verify")
}

func TestNewSystem_CustomCompilerOptions(t *testing.T) {
	sys, err := devmode.NewSystem(
		compilers.ConfigurePCS(
			pcs.WithFRINumQueries(4),
			pcs.WithColumnElision(),
			pcs.WithOptimalMerkleCap(),
		),
	)
	require.NoError(t, err)

	proof, pub, err := devmode.Prove(sys, samplePub())
	require.NoError(t, err)
	require.NoError(t, devmode.Verify(sys, proof, pub))
}

func TestVerify_ChangedPublicInput(t *testing.T) {
	sys, err := devmode.NewSystem()
	require.NoError(t, err)

	proof, pub, err := devmode.Prove(sys, samplePub())
	require.NoError(t, err)

	changed := append(wiop.PublicInput(nil), pub...)
	changed[3] = changed[3].Add(field.ElemOne())
	require.Error(t, devmode.Verify(sys, proof, changed), "a changed public input must be rejected")
	require.Error(t, devmode.Verify(sys, proof, pub[:1]), "a missing public input must be rejected")

	_, _, err = devmode.Prove(sys, pub[:1])
	require.Error(t, err, "Prove must refuse a statement of the wrong length")
}

func TestEncode(t *testing.T) {
	sys, err := devmode.NewSystem()
	require.NoError(t, err)

	proof, pub, err := devmode.Prove(sys, samplePub())
	require.NoError(t, err)

	img, err := devmode.Encode(sys, proof, pub, devmode.GuestBase)
	require.NoError(t, err)
	t.Logf("single devmode proof image size: %d bytes", len(img))

	decoded, err := ps.Decode(img, devmode.GuestBase)
	require.NoError(t, err)
	require.Empty(t, decoded.Proof.ModuleSizes)
	require.Len(t, decoded.PublicInputs, devmode.NumPublicInputs)
	for i := range pub {
		require.Equal(t, ps.ScalarFrom(pub[i]), decoded.PublicInputs[i], "public input %d is X_%d", i, i)
	}
}
