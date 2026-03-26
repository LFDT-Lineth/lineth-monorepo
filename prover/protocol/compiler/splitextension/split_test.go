package splitextension_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitextension"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	// name of the test-case to recognize it
	name string
	// original vector that we want to split defined over E4
	Columns []smartvectors.SmartVector
	X       fext.Element
}

var testCases = []testCase{
	{
		name: "1-fr-vector-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVec(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
	{
		name: "1-fext-vector-with-fr-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromUintBase(2),
	},
	{
		name: "3-fext-vectors-with-fr-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromUintBase(2),
	},
	{
		name: "2-fext-vectors-and-1-fr-vector-with-fr-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
			testtools.RandomVec(64),
		},
		X: fext.NewFromUintBase(2),
	},
	{
		name: "1-fr-vector-and-2-fext-vectors-with-fr-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVec(64),
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromUintBase(2),
	},
	{
		name: "fext-sandwich-with-fr-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVec(64),
			testtools.RandomVecFext(64),
			testtools.RandomVec(64),
		},
		X: fext.NewFromUintBase(2),
	},
	{
		name: "fr-field-sandwich-with-fr-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVec(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromUintBase(2),
	},
	{
		name: "2-fext-vectors-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
	{
		name: "3-fext-vectors-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
	{
		name: "2-fext-vectors-and-1-fr-vector-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
			testtools.RandomVec(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
	{
		name: "1-fr-vector-and-2-fext-vectors-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVec(64),
			testtools.RandomVecFext(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
	{
		name: "fext-sandwich-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVec(64),
			testtools.RandomVecFext(64),
			testtools.RandomVec(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
	{
		name: "fr-field-sandwich-with-fext-x",
		Columns: []smartvectors.SmartVector{
			testtools.RandomVecFext(64),
			testtools.RandomVec(64),
			testtools.RandomVecFext(64),
		},
		X: fext.NewFromInt(1, 2, 3, 4),
	},
}

func TestSplitextension(t *testing.T) {

	// size of the columns
	var (
		baseNameToSplit = "to_split"
		qName           = ifaces.QueryID("q")
	)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			numColumns := len(testCase.Columns)

			define := func(b *wizard.Builder) {
				toSplit := make([]ifaces.Column, numColumns)
				for i := 0; i < len(testCase.Columns); i++ {
					curName := fmt.Sprintf("%s_%d", baseNameToSplit, i)
					toSplit[i] = b.CompiledIOP.InsertCommit(
						0,
						ifaces.ColID(curName),
						testCase.Columns[i].Len(),
						smartvectors.IsBase(testCase.Columns[i]),
					)
				}

				b.InsertUnivariate(0, qName, toSplit)
			}

			genWitness := func(run *wizard.ProverRuntime) {
				y := make([]fext.Element, numColumns)
				for i := 0; i < numColumns; i++ {
					curName := fmt.Sprintf("%s_%d", baseNameToSplit, i)
					run.AssignColumn(ifaces.ColID(curName), testCase.Columns[i])
					y[i] = smartvectors.EvaluateFextPolyLagrange(testCase.Columns[i], testCase.X)
				}
				run.AssignUnivariateExt(qName, testCase.X, y...)
			}

			comp := wizard.Compile(define, splitextension.CompileSplitExtToBase, dummy.Compile)
			proof := wizard.Prove(comp, genWitness)
			assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
		})
	}
}
