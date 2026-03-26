package reedsolomon_test

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/reedsolomon"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestReedSolomon(t *testing.T) {

	wp := smartvectors.ForTest(1, 2, 4, 8, 16, 32, 64, 128, 0, 0, 0, 0, 0, 0, 0, 0)

	domain := fft.NewDomain(uint64(wp.Len()), fft.WithCache())
	v := make([]field.Element, wp.Len())
	wp.WriteInSlice(v)
	domain.FFT(v, fft.DIF, fft.WithNbTasks(1))
	utils.BitReverse(v)
	wp = smartvectors.NewRegular(v)
	// Now wp contains the coefficients of the polynomial

	definer := func(b *wizard.Builder) {
		p := b.RegisterCommit("P", wp.Len())
		reedsolomon.CheckReedSolomon(b.CompiledIOP, 2, p)
	}

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("P", wp)
	}

	compiled := wizard.Compile(definer,
		compiler.Arcane(compiler.WithStitcherMinSize(8), compiler.WithTargetColSize(8)),
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, prover)
	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)

}
