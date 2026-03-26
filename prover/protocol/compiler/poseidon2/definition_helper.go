package poseidon2

import (
	"fmt"

	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"

	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// checkPoseidon2BlockCompressionExpression applies the Poseidon2 block compression function to
// a given block over a given state. The function is as [poseidon2BlockCompression]
// but with [ifaces.Column]. The function does not register prover actions.
//
// The output of the poseidon compression function is checked against the
// output block. The function returns the intermediate columns.
func checkPoseidon2BlockCompressionExpression(comp *wizard.CompiledIOP, oldState, block, output []*symbolic.Expression) [][]ifaces.Column {

	state := make([]*symbolic.Expression, width)
	copy(state[:8], oldState[:])
	copy(state[8:], block[:])

	newState := make([]*symbolic.Expression, len(output))
	copy(newState[:], state[8:])

	// Initial round
	state = matMulExternalExpression(state)

	interm := make([][]ifaces.Column, fullRounds)
	counter := 0
	// External rounds
	for round := 1; round < 1+partialRounds; round++ {
		state = addRoundKeyExpression(round-1, state)
		state = sBoxFullExpression(state)
		state = matMulExternalExpression(state)
		cols := anchorColumns(comp, fmt.Sprintf("POSEIDON2_ROUND_%v_%v", comp.SelfRecursionCount, counter), state)
		state = asExprs(cols)
		interm[counter] = cols
		counter++

	}

	// Internal rounds
	// The Key Optimizations: avoid the high cost of creating new columns every single round, but also reset the expressions before their fan-in becomes high enough to slow down the prover
	//
	// - 1. Partial S-Box: instead of constraining the full S-Box every round, we only apply it on the first element of the state.
	// For most internal rounds, we generate 1 constraint/column instead of 16.
	// This dramatically reduces the total number of columns and constraints the compiler has to manage.
	// - 2. Batched Anchoring: instead of anchoring every round, we do it every `internalPackedSize` rounds. we fully anchor the entire state every internalPackedSize rounds
	// (e.g., the optimial choice is every 3 rounds), we "collapse" the growing linear expressions for state[1] through state[15] back into simple variables.
	for round := 1 + partialRounds; round < fullRounds-partialRounds; round++ {
		state = addRoundKeyExpression(round-1, state)
		state[0] = sBoxPartialExpression(state[0])

		if round%internalPackedSize == 0 {
			cols := anchorColumns(comp, fmt.Sprintf("POSEIDON2_ROUND_%v_%v", comp.SelfRecursionCount, counter), state)
			state = asExprs(cols)
			interm[counter] = cols
			counter++
		} else if partialSBox {
			// Constrain only the first one
			col := anchorSingleColumn(comp, fmt.Sprintf("POSEIDON2_ROUND_%v_%v", comp.SelfRecursionCount, counter), state[0])
			state[0] = symbolic.NewVariable(col)
			interm[counter] = []ifaces.Column{col}
			counter++
		}

		state = matMulInternalExpression(state)

	}

	// External rounds
	for round := fullRounds - partialRounds; round < fullRounds; round++ {
		state = addRoundKeyExpression(round-1, state)
		state = sBoxFullExpression(state)
		state = matMulExternalExpression(state)

		if round < fullRounds-1 {
			cols := anchorColumns(comp, fmt.Sprintf("POSEIDON2_ROUND_%v_%v", comp.SelfRecursionCount, counter), state)
			state = asExprs(cols)
			interm[counter] = cols
			counter++
		}
	}

	// Final round; feed-forward and compare against output
	_, round, _ := wizardutils.AsExpr(newState[0])
	for i := range newState {
		newState[i] = symbolic.Add(newState[i], state[8+i])

		comp.InsertGlobal(
			round,
			ifaces.QueryIDf("POSEIDON2_OUTPUT_%v_%v", comp.SelfRecursionCount, i),
			symbolic.Sub(newState[i], output[i]),
		)
	}

	return interm
}

// sBoxFullExpression applies the full s-box over an array of expression.
func sBoxFullExpression(input []*symbolic.Expression) []*symbolic.Expression {
	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}

	res := make([]*symbolic.Expression, width)

	for i := range input {
		res[i] = symbolic.Mul(input[i], input[i], input[i])
	}

	return res
}

// sBoxPartialExpression applies the partial s-box over an array of expression.
func sBoxPartialExpression(input *symbolic.Expression) *symbolic.Expression {
	res := symbolic.Mul(input, input, input)
	return res
}

// addRoundKeyExpression applies the round key addition of poseidon 2 over the
// provided 16-length array of expressions. The input array is not modified.
func addRoundKeyExpression(round int, input []*symbolic.Expression) []*symbolic.Expression {

	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}

	addRoundKey := make([]*symbolic.Expression, width)

	for i := 0; i < len(gnarkposeidon2.GetDefaultParameters().RoundKeys[round]); i++ {
		addRoundKey[i] = symbolic.Add(input[i], gnarkposeidon2.GetDefaultParameters().RoundKeys[round][i])
	}

	for i := len(gnarkposeidon2.GetDefaultParameters().RoundKeys[round]); i < width; i++ {
		addRoundKey[i] = input[i]
	}

	return addRoundKey
}

// matMulInternalExpression applies the internal matrix multiplication of
// poseidon 2 over the provided 16-length array of expressions. The input array
// is not modified.
func matMulInternalExpression(input []*symbolic.Expression) []*symbolic.Expression {

	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}
	matMulInternal := make([]*symbolic.Expression, 16)

	sBoxSum := input[0]
	for i := 1; i < width; i++ {
		sBoxSum = symbolic.Add(sBoxSum, input[i])
	}

	// mul by diag16:
	// [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
	half := symbolic.NewConstant(1065353217)
	halfExp3 := symbolic.NewConstant(1864368129)
	halfExp4 := symbolic.NewConstant(1997537281)
	halfExp8 := symbolic.NewConstant(2122383361)
	halfExp24 := symbolic.NewConstant(127) // -127

	matMulInternal[0] = symbolic.Sub(sBoxSum, input[0], input[0])
	matMulInternal[1] = symbolic.Add(sBoxSum, input[1])
	matMulInternal[2] = symbolic.Add(sBoxSum, input[2], input[2])
	matMulInternal[3] = symbolic.Add(sBoxSum, symbolic.Mul(input[3], half))
	matMulInternal[4] = symbolic.Add(sBoxSum, symbolic.Mul(input[4], 3))
	matMulInternal[5] = symbolic.Add(sBoxSum, symbolic.Mul(input[5], 4))
	matMulInternal[6] = symbolic.Sub(sBoxSum, symbolic.Mul(input[6], half))
	matMulInternal[7] = symbolic.Sub(sBoxSum, symbolic.Mul(input[7], 3))
	matMulInternal[8] = symbolic.Sub(sBoxSum, symbolic.Mul(input[8], 4))
	matMulInternal[9] = symbolic.Add(sBoxSum, symbolic.Mul(input[9], halfExp8))
	matMulInternal[10] = symbolic.Add(sBoxSum, symbolic.Mul(input[10], halfExp3))
	matMulInternal[11] = symbolic.Sub(sBoxSum, symbolic.Mul(input[11], halfExp24))
	matMulInternal[12] = symbolic.Sub(sBoxSum, symbolic.Mul(input[12], halfExp8))
	matMulInternal[13] = symbolic.Sub(sBoxSum, symbolic.Mul(input[13], halfExp3))
	matMulInternal[14] = symbolic.Sub(sBoxSum, symbolic.Mul(input[14], halfExp4))
	matMulInternal[15] = symbolic.Add(sBoxSum, symbolic.Mul(input[15], halfExp24))

	return matMulInternal
}

// matMulExpression takes an array of expressions as input and returns a list of
// expressions which are the result of the external matrix multiplication of
// poseidon 2.
//
// The input array is expected to contain 16 non-nil expressions exactly,
// otherwise it will panic.
func matMulExternalExpression(input []*symbolic.Expression) []*symbolic.Expression {

	if len(input) != width {
		utils.Panic("matMulExpression must be called with 16 columns, got %v", len(input))
	}

	var (
		matMulM4Tmp    = makeArrayOfZeroes(matMulM4TmpSize)
		matMulM4       = makeArrayOfZeroes(width)
		t              = makeArrayOfZeroes(tSize)
		matMulExternal = makeArrayOfZeroes(width)
	)

	for i := 0; i < 4; i++ {
		matMulM4Tmp[5*i] = symbolic.Add(input[4*i], input[4*i+1])
		matMulM4Tmp[5*i+1] = symbolic.Add(input[4*i+2], input[4*i+3])
		matMulM4Tmp[5*i+2] = symbolic.Add(matMulM4Tmp[5*i], matMulM4Tmp[5*i+1])
		matMulM4Tmp[5*i+3] = symbolic.Add(matMulM4Tmp[5*i+2], input[4*i+1])
		matMulM4Tmp[5*i+4] = symbolic.Add(matMulM4Tmp[5*i+2], input[4*i+3])

		// The order here is important. Need to overwrite x[0] and x[2] after x[1] and x[3].
		matMulM4[4*i+3] = symbolic.Add(input[4*i], input[4*i], matMulM4Tmp[5*i+4])
		matMulM4[4*i+1] = symbolic.Add(input[4*i+2], input[4*i+2], matMulM4Tmp[5*i+3])
		matMulM4[4*i] = symbolic.Add(matMulM4Tmp[5*i], matMulM4Tmp[5*i+3])
		matMulM4[4*i+2] = symbolic.Add(matMulM4Tmp[5*i+1], matMulM4Tmp[5*i+4])
	}

	for i := 0; i < 4; i++ {
		t[0] = symbolic.Add(t[0], matMulM4[4*i])
		t[1] = symbolic.Add(t[1], matMulM4[4*i+1])
		t[2] = symbolic.Add(t[2], matMulM4[4*i+2])
		t[3] = symbolic.Add(t[3], matMulM4[4*i+3])
	}

	for i := 0; i < 4; i++ {
		matMulExternal[4*i] = symbolic.Add(matMulM4[4*i], t[0])
		matMulExternal[4*i+1] = symbolic.Add(matMulM4[4*i+1], t[1])
		matMulExternal[4*i+2] = symbolic.Add(matMulM4[4*i+2], t[2])
		matMulExternal[4*i+3] = symbolic.Add(matMulM4[4*i+3], t[3])
	}

	return matMulExternal
}

// anchorColumns creates a list of new column and constrains it be equal to the
// provided list of columns. The function returns the list of new columns but
// does not auto-assign them. This is left for the caller to do.
//
// The columns are named as <name>_<index>. The round and the size are inferred
// from the list of input expressions.
func anchorColumns(comp *wizard.CompiledIOP, name string, columns []*symbolic.Expression) []ifaces.Column {

	_, round, size := wizardutils.AsExpr(columns[0])

	news := make([]ifaces.Column, len(columns))
	for i, column := range columns {
		news[i] = comp.InsertCommit(
			round,
			ifaces.ColIDf("%v_%v", name, i),
			size,
			true,
		)

		comp.InsertGlobal(
			round,
			ifaces.QueryIDf("%v_%v_GLOBAL", name, i),
			symbolic.Sub(column, news[i]),
		)
	}

	return news
}

// asExprs is a utility function converting an array of columns into an array of
// expressions.
func asExprs(columns []ifaces.Column) []*symbolic.Expression {
	res := make([]*symbolic.Expression, len(columns))
	for i, column := range columns {
		res[i] = symbolic.NewVariable(column)
	}
	return res
}

func anchorSingleColumn(comp *wizard.CompiledIOP, name string, column *symbolic.Expression) ifaces.Column {

	_, round, size := wizardutils.AsExpr(column)

	news := comp.InsertCommit(
		round,
		ifaces.ColIDf("%v_%v", name, 0),
		size,
		true,
	)

	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("%v_%v_GLOBAL", name, 0),
		symbolic.Sub(column, news),
	)

	return news
}

// makeArrayOfZeroes returns an array of zero expressions.
func makeArrayOfZeroes(size int) []*symbolic.Expression {
	res := make([]*symbolic.Expression, size)
	for i := range res {
		res[i] = symbolic.NewConstant(0)
	}
	return res
}
