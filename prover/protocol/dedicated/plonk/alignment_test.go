package plonk

import (
	"os"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

const nbInputInstances = 3
const nbCircuitInstances = 2

func TestAlignment(t *testing.T) {
	f, err := os.Open("testdata/alignment.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal(err)
	}
	var toAlign *CircuitAlignmentInput
	var alignment *Alignment
	// plonk in wizard is deferred, so we need to capture the runtime to be able
	// to verify concatenated PI column assignment
	var runLeaked *wizard.ProverRuntime
	var inputFillerKey = "alignment-test-input-filler"

	RegisterInputFiller(
		inputFillerKey,
		func(circuitInstance, inputIndex int) field.Element {
			return field.NewElement(uint64(inputIndex + 1))
		})

	cmp := wizard.Compile(func(build *wizard.Builder) {
		toAlign = &CircuitAlignmentInput{
			Name:               "ALIGNMENT_TEST",
			Circuit:            &DummyAlignmentCircuit{Instances: make([]DummyAlignmentCircuitInstance, nbInputInstances)},
			DataToCircuit:      ct.GetCommit(build, "ALIGNMENT_TEST_DATA"),
			DataToCircuitMask:  ct.GetCommit(build, "ALIGNMENT_TEST_DATA_MASK"),
			NbCircuitInstances: nbCircuitInstances,
			InputFillerKey:     inputFillerKey,
		}
		alignment = DefineAlignment(build.CompiledIOP, toAlign)
	}, dummy.Compile)
	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {
		runLeaked = run
		ct.Assign(run, toAlign.DataToCircuit, toAlign.DataToCircuitMask)
		alignment.Assign(run)
	})

	ct.CheckAssignmentCols(runLeaked,
		alignment.IsActive,
		alignment.CircuitInput,
		alignment.ActualCircuitInputMask.Natural,
	)

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}

// DummyAlignmentCircuit is a dummy circuit for testing alignment. It doesn't do
// anything except check that the inputs are in order.
type DummyAlignmentCircuit struct {
	Instances []DummyAlignmentCircuitInstance `gnark:",public"`
}

type DummyAlignmentCircuitInstance struct {
	Vars [6]frontend.Variable `gnark:",public"` // non-power of two to test padding. public input for asserting inputs only
}

func (c *DummyAlignmentCircuit) Define(api frontend.API) error {
	var counter frontend.Variable = 1
	for i := range c.Instances {
		for j := range c.Instances[i].Vars {
			api.AssertIsEqual(c.Instances[i].Vars[j], counter)
			counter = api.Add(counter, 1)
		}
	}
	return nil
}
