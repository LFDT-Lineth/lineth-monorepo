package logderivativesum

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalLogDerivativeDebug(t *testing.T) {

	var sizeA, sizeB int = 2, 2

	define := func(b *wizard.Builder) {
		cola := b.RegisterCommit("A", sizeA)
		colb := b.RegisterCommit("B", sizeB)
		filterA := b.RegisterCommit("filterA", sizeA)
		filterB := b.RegisterCommit("filterB", sizeB)
		//check if colB filtered by filterB is included in colA filtered by filterA
		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{cola}, []ifaces.Column{colb}, filterA, filterB)
	}

	prover := func(run *wizard.ProverRuntime) {
		// assign a and b
		cola := smartvectors.ForTest(0, 1)
		colb := smartvectors.ForTest(0, 1)
		filterA := smartvectors.ForTest(1, 1)
		filterB := smartvectors.ForTest(0, 1)

		run.AssignColumn("A", cola)
		run.AssignColumn("B", colb)

		run.AssignColumn("filterA", filterA)
		run.AssignColumn("filterB", filterB)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestConditionalLogDerivativeLookupSimple(t *testing.T) {

	var sizeA, sizeB int = 16, 8

	define := func(b *wizard.Builder) {
		cola := b.RegisterCommit("A", sizeA)
		colb := b.RegisterCommit("B", sizeB)
		filterA := b.RegisterCommit("filterA", sizeA)
		filterB := b.RegisterCommit("filterB", sizeB)
		//check if colB filtered by filterB is included in colA filtered by filterA
		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{cola}, []ifaces.Column{colb}, filterA, filterB)
	}

	prover := func(run *wizard.ProverRuntime) {
		// assign a and b
		cola := smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15)
		colb := smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7)
		filterA := smartvectors.ForTest(0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0)
		filterB := smartvectors.ForTest(0, 0, 1, 1, 1, 1, 1, 1)

		run.AssignColumn("A", cola)
		run.AssignColumn("B", colb)

		run.AssignColumn("filterA", filterA)
		run.AssignColumn("filterB", filterB)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestConditionalLogDerivativeLookupSimple2(t *testing.T) {

	var sizeA, sizeB int = 16, 4
	var runtime *wizard.ProverRuntime

	define := func(b *wizard.Builder) {
		cola := b.RegisterCommit("S", sizeA)
		colb := b.RegisterCommit("T", sizeB)
		filterA := b.RegisterCommit("filterA", sizeA)
		filterB := b.RegisterCommit("filterB", sizeB)
		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{colb}, []ifaces.Column{cola}, filterB, filterA)
	}

	prover := func(run *wizard.ProverRuntime) {
		runtime = run
		// assign a and b
		cola := smartvectors.ForTest(1, 1, 1, 2, 3, 3, 0, 1, 1, 1, 1, 2, 3, 0, 3, 1)
		colb := smartvectors.ForTest(0, 1, 2, 3)
		filterA := smartvectors.ForTest(1, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1)
		filterB := smartvectors.ForTest(1, 1, 0, 1)
		// m expected to be
		run.AssignColumn("S", cola)
		run.AssignColumn("T", colb)
		run.AssignColumn("filterA", filterA)
		run.AssignColumn("filterB", filterB)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)

	//filtered cola := smartvectors.ForTest(1, -, -, -, 3, 3, -, -, -, -, -, -, 3, 0, 3, 1)
	//filtered colb := colb := smartvectors.ForTest(0, 1, -, 3)

	// cola included in colb, m should be (1, 2, 0, 4)
	// m should be 0, 2, 12, 2) if filtered entries are counted as 0, otherwise
	// in our implementation, entries that should be filtered out are counted as appearances of 0
	// if 0 appears in filteredT=HadamardProduct(T,filterT), the multiplicity vector stores the multiplicity of 0
	// in the last appearance of 0 in filteredT
	expectedM := smartvectors.ForTest(1, 2, 0, 4)
	t.Logf("the list of columns is: %v", runtime.Columns.ListAllKeys())
	actualM := runtime.GetColumn("TABLE_filterB,T_0_LOGDERIVATIVE_M")

	assert.Equal(t, expectedM.Pretty(), actualM.Pretty(), "m does not match the expected value")

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestConditionalLogDerivativeLookupManyChecksOneTable(t *testing.T) {

	var sizeA, sizeB int = 16, 4
	var runtime *wizard.ProverRuntime

	define := func(b *wizard.Builder) {
		cola := b.RegisterCommit("S", sizeA)
		cola2 := b.RegisterCommit("S2", sizeA)
		colb := b.RegisterCommit("T", sizeB)
		filter1 := b.RegisterCommit("filter1", sizeA)
		filter2 := b.RegisterCommit("filter2", sizeA)
		filterT := b.RegisterCommit("filterT", sizeB)
		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{colb}, []ifaces.Column{cola}, filterT, filter1)
		b.InclusionDoubleConditional("LOOKUP2", []ifaces.Column{colb}, []ifaces.Column{cola2}, filterT, filter2)
	}
	prover := func(run *wizard.ProverRuntime) {
		runtime = run
		// assign a and b
		cola := smartvectors.ForTest(1, 1, 1, 2, 3, 0, 0, 1, 1, 1, 1, 2, 3, 0, 0, 1)
		cola2 := smartvectors.ForTest(2, 2, 2, 1, 0, 3, 3, 2, 2, 2, 2, 1, 0, 3, 3, 2)
		colb := smartvectors.ForTest(0, 1, 2, 3) //including column
		//define filters
		filter1 := smartvectors.ForTest(1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1)
		//filtered cola := smartvectors.ForTest(1, 1, -, -, -, -, -, -, -, -, -, -, 3, -, -, 1)
		// 1—appears 3 times, 3 appears once
		filter2 := smartvectors.ForTest(1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1)
		//filtered cola2 := smartvectors.ForTest(2, 2, 2, 1, -, -, -, -, -, -, -, -, -, 3, 3, 2)
		// 1—appears once, 2—appears 4 times, 3 appears twice
		filterT := smartvectors.ForTest(0, 1, 1, 1)
		//filtered colb = smartvectors.ForTest(-, 1, 2, 3)
		// m expected to be 1-4 times, 2-4 times, 3- 3 times
		run.AssignColumn("S", cola)
		run.AssignColumn("S2", cola2)
		run.AssignColumn("T", colb)
		run.AssignColumn("filter1", filter1)
		run.AssignColumn("filter2", filter2)
		run.AssignColumn("filterT", filterT)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)

	// m should be
	expectedM := smartvectors.ForTest(0, 4, 4, 3)
	t.Logf("the list of columns is: %v", runtime.Columns.ListAllKeys())
	actualM := runtime.GetColumn("TABLE_filterT,T_0_LOGDERIVATIVE_M")

	assert.Equal(t, expectedM.Pretty(), actualM.Pretty(), "m does not match the expected value")

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestConditionalLogDerivativeLookupOneXor(t *testing.T) {

	var sizeTable, sizeCheckeds int = 16, 8
	var runtime *wizard.ProverRuntime

	// The test uses a lookup over a xor table

	define := func(b *wizard.Builder) {

		xorX := b.RegisterCommit("XOR_TABLE_X", sizeTable)
		xorY := b.RegisterCommit("XOR_TABLE_Y", sizeTable)
		xorXY := b.RegisterCommit("XOR_TABLE_XXORY", sizeTable)

		wX := b.RegisterCommit("WITNESS_X", sizeCheckeds)
		wY := b.RegisterCommit("WITNESS_Y", sizeCheckeds)
		wXY := b.RegisterCommit("WITNESS_XXORY", sizeCheckeds)

		filterT := b.RegisterCommit("filterT", sizeTable)
		filterS := b.RegisterCommit("filterS", sizeCheckeds)

		//check that witness is included in the XOR table
		//all rows of the witness must be included int he rows of the larger matrix
		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{xorX, xorY, xorXY}, []ifaces.Column{wX, wY, wXY}, filterT, filterS)
	}

	prover := func(run *wizard.ProverRuntime) {
		runtime = run
		// assign a and b
		xorX := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11)
		xorY := smartvectors.ForTest(0b00, 0b00, 0b00, 0b00, 0b01, 0b01, 0b01, 0b01, 0b10, 0b10, 0b10, 0b10, 0b11, 0b11, 0b11, 0b11)
		xorXY := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b01, 0b00, 0b11, 0b10, 0b10, 0b11, 0b00, 0b01, 0b11, 0b10, 0b01, 0b00)

		wX := smartvectors.ForTest(0b00, 0b11, 0b10, 0b01, 0b10, 0b10, 0b01, 0b00)
		wY := smartvectors.ForTest(0b01, 0b00, 0b11, 0b10, 0b00, 0b00, 0b00, 0b01)
		wXY := smartvectors.ForTest(0b01, 0b11, 0b01, 0b11, 0b10, 0b10, 0b01, 0b01)

		filterT := smartvectors.ForTest(1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1)
		filterS := smartvectors.ForTest(1, 1, 1, 1, 1, 0, 1, 1)

		run.AssignColumn("XOR_TABLE_X", xorX)
		run.AssignColumn("XOR_TABLE_Y", xorY)
		run.AssignColumn("XOR_TABLE_XXORY", xorXY)

		run.AssignColumn("WITNESS_X", wX)
		run.AssignColumn("WITNESS_Y", wY)
		run.AssignColumn("WITNESS_XXORY", wXY)

		run.AssignColumn("filterT", filterT)
		run.AssignColumn("filterS", filterS)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)

	// m should be
	expectedM := smartvectors.ForTest(0, 1, 1, 1, 2, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0)

	t.Logf("the list of columns is: %v", runtime.Columns.ListAllKeys())
	actualM := runtime.GetColumn("TABLE_filterT,XOR_TABLE_X,XOR_TABLE_XXORY,XOR_TABLE_Y_0_LOGDERIVATIVE_M")

	assert.Equal(t, expectedM.Pretty(), actualM.Pretty(), "m does not match the expected value")

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestConditionalLogDerivativeLookupMultiXor(t *testing.T) {

	var sizeTable, sizeCheckeds, sizeCheckedLarger int = 16, 4, 8
	var runtime *wizard.ProverRuntime

	// The test uses a lookup over a xor table

	define := func(b *wizard.Builder) {

		xorX := b.RegisterCommit("XOR_TABLE_X", sizeTable)
		xorY := b.RegisterCommit("XOR_TABLE_Y", sizeTable)
		xorXY := b.RegisterCommit("XOR_TABLE_XXORY", sizeTable)

		wX := b.RegisterCommit("WITNESS_X", sizeCheckeds)
		wY := b.RegisterCommit("WITNESS_Y", sizeCheckeds)
		wXY := b.RegisterCommit("WITNESS_XXORY", sizeCheckeds)

		w2X := b.RegisterCommit("W2_X", sizeCheckedLarger)
		w2Y := b.RegisterCommit("W2_Y", sizeCheckedLarger)
		w2XY := b.RegisterCommit("W2_XXORY", sizeCheckedLarger)

		filterT := b.RegisterCommit("FILTER_T", sizeTable)
		filterS1 := b.RegisterCommit("FILTER_S1", sizeCheckeds)
		filterS2 := b.RegisterCommit("FILTER_S2", sizeCheckedLarger)

		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{xorX, xorY, xorXY}, []ifaces.Column{wX, wY, wXY}, filterT, filterS1)
		b.InclusionDoubleConditional("LOOKUP2", []ifaces.Column{xorX, xorY, xorXY}, []ifaces.Column{w2X, w2Y, w2XY}, filterT, filterS2)

	}

	prover := func(run *wizard.ProverRuntime) {
		runtime = run
		// assign a and b
		xorX := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11)
		xorY := smartvectors.ForTest(0b00, 0b00, 0b00, 0b00, 0b01, 0b01, 0b01, 0b01, 0b10, 0b10, 0b10, 0b10, 0b11, 0b11, 0b11, 0b11)
		xorXY := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b01, 0b00, 0b11, 0b10, 0b10, 0b11, 0b00, 0b01, 0b11, 0b10, 0b01, 0b00)

		wX := smartvectors.ForTest(0b00, 0b11, 0b10, 0b01)
		wY := smartvectors.ForTest(0b01, 0b00, 0b11, 0b10)
		wXY := smartvectors.ForTest(0b01, 0b11, 0b01, 0b11)

		w2X := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11)
		w2Y := smartvectors.ForTest(0b00, 0b00, 0b00, 0b00, 0b01, 0b01, 0b01, 0b01)
		w2XY := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b01, 0b00, 0b11, 0b10)

		filterT := smartvectors.ForTest(1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1)
		filterS1 := smartvectors.ForTest(0, 1, 1, 0)
		filterS2 := smartvectors.ForTest(1, 1, 1, 1, 0, 0, 1, 1)

		run.AssignColumn("XOR_TABLE_X", xorX)
		run.AssignColumn("XOR_TABLE_Y", xorY)
		run.AssignColumn("XOR_TABLE_XXORY", xorXY)

		run.AssignColumn("WITNESS_X", wX)
		run.AssignColumn("WITNESS_Y", wY)
		run.AssignColumn("WITNESS_XXORY", wXY)

		run.AssignColumn("W2_X", w2X)
		run.AssignColumn("W2_Y", w2Y)
		run.AssignColumn("W2_XXORY", w2XY)

		run.AssignColumn("FILTER_T", filterT)
		run.AssignColumn("FILTER_S1", filterS1)
		run.AssignColumn("FILTER_S2", filterS2)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)

	// m should be
	expectedM := smartvectors.ForTest(1, 1, 1, 2, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 0)
	t.Logf("the list of columns is: %v", runtime.Columns.ListAllKeys())
	actualM := runtime.GetColumn("TABLE_FILTER_T,XOR_TABLE_X,XOR_TABLE_XXORY,XOR_TABLE_Y_0_LOGDERIVATIVE_M")

	assert.Equal(t, expectedM.Pretty(), actualM.Pretty(), "m does not match the expected value")

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

/*
the following test mixes different types of conditional and non-conditional lookups
*/
func TestMixedConditionalLogDerivativeLookupMultiXor(t *testing.T) {

	var sizeTable, sizeCheckeds, sizeCheckedLarger int = 16, 4, 8
	var runtime *wizard.ProverRuntime

	// The test uses a lookup over a xor table

	define := func(b *wizard.Builder) {

		xorX := b.RegisterCommit("XOR_TABLE_X", sizeTable)
		xorY := b.RegisterCommit("XOR_TABLE_Y", sizeTable)
		xorXY := b.RegisterCommit("XOR_TABLE_XXORY", sizeTable)

		wX := b.RegisterCommit("WITNESS_X", sizeCheckeds)
		wY := b.RegisterCommit("WITNESS_Y", sizeCheckeds)
		wXY := b.RegisterCommit("WITNESS_XXORY", sizeCheckeds)

		w2X := b.RegisterCommit("W2_X", sizeCheckedLarger)
		w2Y := b.RegisterCommit("W2_Y", sizeCheckedLarger)
		w2XY := b.RegisterCommit("W2_XXORY", sizeCheckedLarger)

		w3X := b.RegisterCommit("W3_X", sizeCheckeds)
		w3Y := b.RegisterCommit("W3_Y", sizeCheckeds)
		w3XY := b.RegisterCommit("W3_XXORY", sizeCheckeds)

		w4X := b.RegisterCommit("W4_X", sizeCheckeds)
		w4Y := b.RegisterCommit("W4_Y", sizeCheckeds)
		w4XY := b.RegisterCommit("W4_XXORY", sizeCheckeds)

		filterT := b.RegisterCommit("FILTER_T", sizeTable)
		filterS1 := b.RegisterCommit("FILTER_S1", sizeCheckeds)
		filterS2 := b.RegisterCommit("FILTER_S2", sizeCheckedLarger)
		filterS4 := b.RegisterCommit("FILTER_S4", sizeCheckeds)

		b.InclusionDoubleConditional("LOOKUP", []ifaces.Column{xorX, xorY, xorXY}, []ifaces.Column{wX, wY, wXY}, filterT, filterS1)
		b.InclusionDoubleConditional("LOOKUP2", []ifaces.Column{xorX, xorY, xorXY}, []ifaces.Column{w2X, w2Y, w2XY}, filterT, filterS2)
		b.InclusionConditionalOnIncluding("LOOKUP3", []ifaces.Column{xorX, xorY, xorXY}, []ifaces.Column{w3X, w3Y, w3XY}, filterT)
		constantOne := verifiercol.NewConstantCol(field.One(), sizeCheckeds, "")
		b.Inclusion("LOOKUP4", []ifaces.Column{xorX, xorY, xorXY, filterT}, []ifaces.Column{w4X, w4Y, w4XY, constantOne})
		// next query will use w4X, w4Y, w4XY again (to prevent the code from getting too verbose)
		b.InclusionConditionalOnIncluded("LOOKUP5", []ifaces.Column{xorX, xorY, xorXY, filterT}, []ifaces.Column{w4X, w4Y, w4XY, constantOne}, filterS4)
	}

	prover := func(run *wizard.ProverRuntime) {
		runtime = run
		// assign a and b
		xorX := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11)
		xorY := smartvectors.ForTest(0b00, 0b00, 0b00, 0b00, 0b01, 0b01, 0b01, 0b01, 0b10, 0b10, 0b10, 0b10, 0b11, 0b11, 0b11, 0b11)
		xorXY := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b01, 0b00, 0b11, 0b10, 0b10, 0b11, 0b00, 0b01, 0b11, 0b10, 0b01, 0b00)

		wX := smartvectors.ForTest(0b00, 0b11, 0b10, 0b01)
		wY := smartvectors.ForTest(0b01, 0b00, 0b11, 0b10)
		wXY := smartvectors.ForTest(0b01, 0b11, 0b01, 0b11)

		w2X := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b00, 0b01, 0b10, 0b11)
		w2Y := smartvectors.ForTest(0b00, 0b00, 0b00, 0b00, 0b01, 0b01, 0b01, 0b01)
		w2XY := smartvectors.ForTest(0b00, 0b01, 0b10, 0b11, 0b01, 0b00, 0b11, 0b10)

		w3X := smartvectors.ForTest(0b00, 0b01, 0b11, 0b00)
		w3Y := smartvectors.ForTest(0b00, 0b00, 0b01, 0b01)
		w3XY := smartvectors.ForTest(0b00, 0b01, 0b10, 0b01)

		w4X := smartvectors.ForTest(0b11, 0b00, 0b01, 0b00)
		w4Y := smartvectors.ForTest(0b01, 0b01, 0b11, 0b11)
		w4XY := smartvectors.ForTest(0b10, 0b01, 0b10, 0b11)

		filterT := smartvectors.ForTest(1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1)
		filterS1 := smartvectors.ForTest(0, 1, 1, 0)
		filterS2 := smartvectors.ForTest(1, 1, 1, 1, 0, 0, 1, 1)
		filterS4 := smartvectors.ForTest(0, 0, 0, 1)

		run.AssignColumn("XOR_TABLE_X", xorX)
		run.AssignColumn("XOR_TABLE_Y", xorY)
		run.AssignColumn("XOR_TABLE_XXORY", xorXY)

		run.AssignColumn("WITNESS_X", wX)
		run.AssignColumn("WITNESS_Y", wY)
		run.AssignColumn("WITNESS_XXORY", wXY)

		run.AssignColumn("W2_X", w2X)
		run.AssignColumn("W2_Y", w2Y)
		run.AssignColumn("W2_XXORY", w2XY)

		run.AssignColumn("W3_X", w3X)
		run.AssignColumn("W3_Y", w3Y)
		run.AssignColumn("W3_XXORY", w3XY)

		run.AssignColumn("W4_X", w4X)
		run.AssignColumn("W4_Y", w4Y)
		run.AssignColumn("W4_XXORY", w4XY)

		run.AssignColumn("FILTER_T", filterT)
		run.AssignColumn("FILTER_S1", filterS1)
		run.AssignColumn("FILTER_S2", filterS2)
		run.AssignColumn("FILTER_S4", filterS4)
	}

	comp := wizard.Compile(define, CompileLookups, dummy.Compile)
	proof := wizard.Prove(comp, prover)

	// m should be
	expectedM := smartvectors.ForTest(2, 2, 1, 2, 2, 0, 1, 3, 0, 0, 0, 0, 2, 1, 1, 0) // 16 rows are included
	t.Logf("the list of columns is: %v", runtime.Columns.ListAllKeys())
	actualM := runtime.GetColumn("TABLE_FILTER_T,XOR_TABLE_X,XOR_TABLE_XXORY,XOR_TABLE_Y_0_LOGDERIVATIVE_M")

	assert.Equal(t, expectedM.Pretty(), actualM.Pretty(), "m does not match the expected value")

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

// testing the method inclusion.check for conditional inclusion
func TestConditionalWithDummyCompilerOnly(t *testing.T) {
	var sizeA, sizeB int = 16, 16

	define := func(b *wizard.Builder) {
		cola := b.RegisterCommit("A", sizeA)
		colb := b.RegisterCommit("B", sizeB)
		filterA := b.RegisterCommit("filterA", sizeA)
		filterB := b.RegisterCommit("filterB", sizeB)
		//check if colB filtered by filterB is included in colA filtered by filterA
		b.InclusionDoubleConditional("LOOKUP1", []ifaces.Column{cola}, []ifaces.Column{colb}, filterA, filterB)
		//b.InclusionDoubleConditional("LOOKUP2", []ifaces.Column{colb}, []ifaces.Column{cola}, filterB, filterA)
	}

	prover := func(run *wizard.ProverRuntime) {
		// assign a and b
		filterA := smartvectors.ForTest(1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
		filterB := smartvectors.ForTest(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1)
		cola := smartvectors.ForTest(1, 2, 3, 4, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6)
		colb := smartvectors.ForTest(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6)

		run.AssignColumn("A", cola)
		run.AssignColumn("B", colb)
		run.AssignColumn("filterA", filterA)
		run.AssignColumn("filterB", filterB)
	}

	comp := wizard.Compile(
		define,
		// CompileLookups,
		dummy.Compile,
	)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}
