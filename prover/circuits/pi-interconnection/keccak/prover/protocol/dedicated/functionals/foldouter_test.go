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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFoldingOuter(t *testing.T) {

	var (
		x            ifaces.Accessor
		savedRuntime *wizard.ProverRuntime
		p            ifaces.Column
		folded       ifaces.Column
		size         int = 32
		outerDegree  int = 4
	)

	/*
		The test consists  in folding a vector of the form
			0, 1, 2, 3, ... n-1
		using the variable 2
	*/
	wpVec := make([]field.Element, size)
	for i := range wpVec {
		wpVec[i] = field.NewElement(uint64(i))
	}
	wp := smartvectors.NewRegular(wpVec)

	definer := func(b *wizard.Builder) {
		p = b.RegisterCommit("P", size)
		x = accessors.NewConstant(field.NewElement(2))
		folded = functionals.FoldOuter(b.CompiledIOP, p, x, outerDegree)

		// Ensures that we are not mistaken with the dimensions
		require.Equal(t, size/outerDegree, folded.Size())
	}

	prover := func(run *wizard.ProverRuntime) {
		// Save the pointer toward the prover
		savedRuntime = run
		run.AssignColumn("P", wp)
	}

	compiled := wizard.Compile(definer, dummy.Compile)
	proof := wizard.Prove(compiled, prover)

	// Computes the expected value
	expected := make([]field.Element, size/outerDegree)
	for i := range expected {
		v := 0
		for j := 0; j < outerDegree; j++ {
			// because we have
			// 		- p[pos] = pos = i * innerDegree + j
			// 		- 2^j = 1 << j
			v += (i + j*size/outerDegree) * (1 << j)
		}
		expected[i].SetUint64(uint64(v))
	}

	actual := smartvectors.IntoRegVec(savedRuntime.GetColumn(folded.GetColID()))
	for i := range expected {
		assert.Equal(t, expected[i], actual[i], "folded does not match")
	}

	err := wizard.Verify(compiled, proof)
	assert.NoError(t, err)
}
