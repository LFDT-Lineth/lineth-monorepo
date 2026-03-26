package functionals_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/require"
)

func TestInterpolate(t *testing.T) {

	var (
		x            ifaces.Accessor
		p            ifaces.Column
		acc          ifaces.Accessor
		savedRuntime *wizard.ProverRuntime
	)

	wpVec := make([]field.Element, 32)
	for i := range wpVec {
		wpVec[i] = field.NewElement(uint64(i))
	}
	wp := smartvectors.NewRegular(wpVec)

	definer := func(b *wizard.Builder) {
		p = b.RegisterCommit("P", wp.Len())
		x = accessors.NewConstant(field.NewElement(2))
		acc = functionals.Interpolation(b.CompiledIOP, "INTERPOLATE", x, p)

	}

	prover := func(run *wizard.ProverRuntime) {
		// Save the pointer toward the prover
		savedRuntime = run
		run.AssignColumn("P", wp)
	}

	compiled := wizard.Compile(definer,
		// specialqueries.RangeProof,
		// specialqueries.CompileFixedPermutations,
		// specialqueries.CompileInclusionPermutations,
		// innerproduct.Compile,
		// splitter.SplitColumns(8),
		// arithmetics.CompileLocal,
		// arithmetics.CompileGlobal,
		dummy.Compile,
	)

	proof := wizard.Prove(compiled, prover)

	xVal := x.GetVal(savedRuntime)
	accY := acc.GetVal(savedRuntime)
	expectedY := smartvectors.Interpolate(wp, xVal)

	require.Equal(t, accY, expectedY)

	err := wizard.Verify(compiled, proof)
	require.NoError(t, err)
}
