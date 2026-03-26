package symbolic

import (
	"runtime/debug"
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/arena"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"

	"github.com/consensys/gnark/frontend"

	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
)

var onceChunkSize sync.Once
var chunkSize int

// ChunkSize is the size of the chunks we use to evaluate the expression board.
// It is determined at runtime depending on the available memory, from
// GOMEMLIMIT>700GiB -> 1<<9
// GOMEMLIMIT<=700GiB -> 1<<8
func ChunkSize() int {
	onceChunkSize.Do(func() {
		// get the memory limit
		memLimit := debug.SetMemoryLimit(-1)

		// if we are <= 700GB, use 1 << 8
		// else use 1 << 9
		if memLimit > 0 && memLimit <= 700*1024*1024*1024 {
			chunkSize = 1 << 8
		} else {
			chunkSize = 1 << 9
		}
	})
	return chunkSize
}

// evaluation is a helper to evaluate an expression board on chunks of data.
type evaluation struct {
	nodes       []evaluationNode
	scratch     []field.Element
	chunkSize   int
	nbInputs    int                // len of level 0
	vectorArena *arena.VectorArena // contiguous memory to store all the vectors
}

// evaluationNode is a node in the evaluation graph.
type evaluationNode struct {
	value      []field.Element // value of the node after evaluation
	inputs     []uint32        // index of the input nodes
	op         Operator        // op(inputs) = value
	hasValue   bool            // true if value is valid
	isConstant bool            // true if the node is a constant
}

// Evaluate evaluates the expression board on the provided inputs.
func (b *ExpressionBoard) Evaluate(inputs []sv.SmartVector, vArena ...*arena.VectorArena) sv.SmartVector {
	// essentially, we can see the inputs as "columns", and the chunks as "rows"
	// the relations between the columns are defined by the expression board
	// we evaluate the expression board chunk by chunk, in parallel.

	if len(inputs) == 0 {
		panic("no input provided")
	}
	// sanity check the inputs
	totalSize := inputs[0].Len()
	for i := 1; i < len(inputs); i++ {
		if inputs[i].Len() != totalSize {
			utils.Panic("mismatch in the size: len(v0) %v, len(v%v) %v", totalSize, i, inputs[i].Len())
		}
	}
	if totalSize == 0 {
		panic("inputs all have size 0")
	}

	chunkSize := min(ChunkSize(), totalSize) // default chunk size
	if totalSize%chunkSize != 0 {
		panic("chunk size should divide total size")
	}

	numChunks := totalSize / chunkSize

	// if all inputs are base, the result is also base
	res := make([]field.Element, totalSize)

	parallel.Execute(numChunks, func(start, stop int) {
		var a *arena.VectorArena
		if len(vArena) == 1 {
			a = vArena[0]
		} else {
			nbNodes := b.CountNodesFilterConstants()
			for _, in := range inputs {
				if _, ok := in.(*sv.Regular); ok {
					nbNodes-- // for each regular , we are not going to alloc
				}
			}
			a = arena.NewVectorArena[field.Element](nbNodes * chunkSize)
		}

		eval := newEvaluation(b, chunkSize, a)
		for chunkID := start; chunkID < stop; chunkID++ {

			chunkStart := chunkID * chunkSize
			chunkStop := (chunkID + 1) * chunkSize

			eval.reset(inputs, chunkStart, chunkStop, a)
			eval.evaluate()

			copy(res[chunkStart:chunkStop], eval.nodes[len(eval.nodes)-1].value[:])
		}

	})

	if areAllConstants(inputs) {
		return sv.NewConstant(res[0], totalSize)
	}

	return sv.NewRegular(res)
}

func newEvaluation(b *ExpressionBoard, chunkSize int, vArena *arena.VectorArena) evaluation {
	eval := evaluation{
		scratch:     make([]field.Element, chunkSize),
		chunkSize:   chunkSize,
		nodes:       make([]evaluationNode, b.CountNodes()),
		nbInputs:    len(b.Nodes[0]),
		vectorArena: vArena,
	}

	// count total number of children to allocate inputs arrays
	totalNbChildren := 0
	for i := range b.Nodes {
		if i == 0 {
			continue // level 0 has no children
		}
		for j := range b.Nodes[i] {
			totalNbChildren += len(b.Nodes[i][j].Children)
		}
	}
	inputsArena := arena.NewVectorArena[uint32](totalNbChildren)

	// Init the constants and inputs.
	inputCursor := 0
	nodeIndex := 0
	for i := range b.Nodes {
		for j := range b.Nodes[i] {
			na := &eval.nodes[nodeIndex]
			node := &b.Nodes[i][j]
			na.op = node.Operator
			na.hasValue = false
			na.isConstant = false
			na.inputs = arena.Get[uint32](inputsArena, len(node.Children))
			nodeIndex++

			switch op := node.Operator.(type) {
			case Constant:
				// The constants are identified to constant vectors
				na.isConstant = true
				na.hasValue = true
				na.value = make([]field.Element, 1)
				na.value[0] = op.Val
			case Variable:
				// we don't allocate for variables since we try to re-use the input vectors
				inputCursor++
			default:
				na.value = arena.Get[field.Element](eval.vectorArena, chunkSize)

			}

			// set the inputs pointers
			for k := range node.Children {
				id := node.Children[k]
				na.inputs[k] = getIndex(id, b) // index of the input node
			}
		}
	}

	return eval
}

func getIndex(id nodeID, b *ExpressionBoard) uint32 {
	// note: quite inefficient, but it's only during initialization
	lvl := id.level()
	pos := id.posInLevel()
	cum := 0
	for i := 0; i < lvl; i++ {
		cum += len(b.Nodes[i])
	}
	return uint32(cum + pos)
}

func (e *evaluation) reset(inputs []sv.SmartVector, chunkStart, chunkStop int, vArena *arena.VectorArena) {
	inputCursor := 0
	chunkLen := chunkStop - chunkStart // can be < MaxChunkSize

	// Init the constants and inputs.
	for i := range e.nodes {
		na := &e.nodes[i]
		if !na.isConstant {
			na.hasValue = false
		}
		if i >= e.nbInputs {
			continue
		}
		switch na.op.(type) {
		case Variable:
			input := inputs[inputCursor]
			switch rv := input.(type) {
			case *sv.Regular:
				na.value = (*rv)[chunkStart:chunkStop]
			case *sv.Rotated:
				if na.value == nil {
					na.value = arena.Get[field.Element](vArena, chunkLen)
				}
				rv.WriteSubVectorInSlice(chunkStart, chunkStop, na.value[:chunkLen])
			case *sv.Constant:
				if !na.hasValue {
					na.value = make([]field.Element, 1)
					na.value[0] = rv.Value
				}
				na.isConstant = true
			default:
				if na.value == nil {
					na.value = arena.Get[field.Element](vArena, chunkLen)
				}
				sb := input.SubVector(chunkStart, chunkStop)
				sb.WriteInSlice(na.value[:chunkLen])
			}

			inputCursor++
			na.hasValue = true
		}
	}
}

func (e *evaluation) evaluate() {
	for i := e.nbInputs; i < len(e.nodes); i++ {
		evalNode(e, &e.nodes[i])
	}
}

func (e *evaluation) getNode(id uint32) *evaluationNode {
	return &e.nodes[id]
}

func evalProduct(vRes, vTmp field.Vector, inputs []uint32, exponents []int, e *evaluation) (isConstant bool) {
	hasConst := false
	constTerm := field.One()
	t0 := field.Element{}
	isResInitialized := false
	for i := 0; i < len(inputs); i++ {
		inputID := inputs[i]
		input := e.nodes[inputID]
		if input.isConstant {
			// accumulate the constant term
			hasConst = true
			field.ExpInt64(&t0, input.value[0], int64(exponents[i]))
			constTerm.Mul(&constTerm, &t0)
			continue
		}
		vInput := field.Vector(input.value[:])
		if !isResInitialized {
			field.ExpVec(vRes, vInput, int64(exponents[i]))
			isResInitialized = true
		} else {
			if exponents[i] == 1 {
				// common case
				vRes.Mul(vRes, vInput)
				continue
			}
			field.ExpVec(vTmp, vInput, int64(exponents[i]))
			vRes.Mul(vRes, vTmp)
		}
	}
	if !isResInitialized {
		// all constant
		for i := range vRes {
			vRes[i] = constTerm
		}
		return true
	}
	if hasConst {
		vRes.ScalarMul(vRes, &constTerm)
	}
	return false
}

func evalNode(e *evaluation, na *evaluationNode) {
	if na.hasValue {
		return
	}

	vRes := field.Vector(na.value[:])
	vTmp := field.Vector(e.scratch[:])

	switch op := na.op.(type) {
	case Product:
		na.isConstant = evalProduct(vRes, vTmp, na.inputs, op.Exponents, e)
		na.hasValue = true
		return

	case LinComb:
		var t0 field.Element
		hasConst := false
		hasNonConst := false
		constTerm := field.Element{}
		for i := range na.inputs {
			if e.getNode(na.inputs[i]).isConstant {
				t0.SetInt64(int64(op.Coeffs[i]))
				t0.Mul(&t0, &e.getNode(na.inputs[i]).value[0])
				constTerm.Add(&constTerm, &t0)
				hasConst = true
			} else {
				hasNonConst = true
			}
		}

		if !hasNonConst {
			// all constant
			for i := range vRes {
				vRes[i] = constTerm
			}
			na.hasValue = true
			na.isConstant = true
			return
		}
		first := true
		for i := range na.inputs {
			if e.getNode(na.inputs[i]).isConstant {
				continue
			}
			vInput := field.Vector(e.getNode(na.inputs[i]).value[:])
			coeff := op.Coeffs[i]
			if first {
				switch coeff {
				case 0:
					for j := range vRes {
						vRes[j].SetZero()
					}
				case 1:
					copy(vRes, vInput)
				case 2:
					vRes.Add(vInput, vInput)
				case -1:
					for j := range vRes {
						vRes[j].SetZero()
					}
					vRes.Sub(vRes, vInput)
				default:
					t0.SetInt64(int64(coeff))
					vRes.ScalarMul(vInput, &t0)
				}
				first = false
				continue
			}

			switch coeff {
			case 0:
				continue
			case 1:
				vRes.Add(vRes, vInput)
			case 2:
				vRes.Add(vRes, vInput)
				vRes.Add(vRes, vInput)
			case -1:
				vRes.Sub(vRes, vInput)
			default:
				t0.SetInt64(int64(op.Coeffs[i]))
				vTmp.ScalarMul(vInput, &t0)
				vRes.Add(vRes, vTmp)
			}
		}
		if hasConst {
			for i := range vRes {
				vRes[i].Add(&vRes[i], &constTerm)
			}
		}

	case PolyEval:
		// result = input[0] + input[1]·x + input[2]·x² + input[3]·x³ + ...
		// i.e., ∑_{i=0}^{n} input[i]·x^i
		x := e.getNode(na.inputs[0]).value[0] // we assume that the first input is always a constant
		lastInput := e.getNode(na.inputs[len(na.inputs)-1])
		if lastInput.isConstant {
			for i := range vRes {
				vRes[i] = lastInput.value[0]
			}
		} else {
			vInput := field.Vector(lastInput.value[:])
			// copy the last input
			copy(vRes, vInput)
		}

		for i := len(na.inputs) - 2; i >= 1; i-- {
			vRes.ScalarMul(vRes, &x)
			if e.getNode(na.inputs[i]).isConstant {
				cst := &e.getNode(na.inputs[i]).value[0]
				for j := range vRes {
					vRes[j].Add(&vRes[j], cst)
				}
			} else {
				vInput := field.Vector(e.getNode(na.inputs[i]).value[:])
				vRes.Add(vRes, vInput)
			}
		}
	default:
		utils.Panic("unknown op %T", na.op)
	}

	na.hasValue = true

}

type GetDegree = func(m interface{}) int

// ListVariableMetadata return the list of the metadata of the variables
// into the board. Importantly, the order in which the metadata is returned
// matches the order in which the assignments to the boarded expression must be
// provided.
func (b *ExpressionBoard) ListVariableMetadata() []Metadata {
	res := []Metadata{}
	for i := range b.Nodes[0] {
		if vari, ok := b.Nodes[0][i].Operator.(Variable); ok {
			res = append(res, vari.Metadata)
		}
	}
	return res
}

// Degree returns the overall degree of the expression board. It admits a custom
// function `getDeg` which is used to assign a degree to the [Variable] leaves
// of the ExpressionBoard.
func (b *ExpressionBoard) Degree(getdeg GetDegree) int {

	/*
		First, build a buffer to store the intermediate results
	*/
	intermediateRes := make([][]int, len(b.Nodes))
	for level := range b.Nodes {
		intermediateRes[level] = make([]int, len(b.Nodes[level]))
	}

	/*
		Then, store the initial values in the level entries of the vector
	*/
	inputCursor := 0

	for i, node := range b.Nodes[0] {
		switch v := node.Operator.(type) {
		case Constant:
			intermediateRes[0][i] = 0
		case Variable:
			intermediateRes[0][i] = getdeg(v.Metadata)
			inputCursor++
		}
	}

	/*
		Then computes the levels one by one
	*/
	for level := 1; level < len(b.Nodes); level++ {
		for pos, node := range b.Nodes[level] {
			/*
				Collect the inputs of the current node from the intermediateRes
			*/
			childrenDeg := make([]int, len(node.Children))
			for i, childID := range node.Children {
				childrenDeg[i] = intermediateRes[childID.level()][childID.posInLevel()]
			}

			/*
				Run the evaluation
			*/
			nodeDeg := node.Operator.Degree(childrenDeg)

			/*
				Registers the result in the intermediate results
			*/
			intermediateRes[level][pos] = nodeDeg
		}
	}

	// Deep-copy the result from the last level (which assumedly contains only one node)
	if len(intermediateRes[len(intermediateRes)-1]) > 1 {
		panic("multiple heads")
	}
	return intermediateRes[len(b.Nodes)-1][0]
}

/*
GnarkEval evaluates the expression in a gnark circuit
*/
func (b *ExpressionBoard) GnarkEval(api frontend.API, inputs []frontend.Variable) frontend.Variable {

	/*
		First, build a buffer to store the intermediate results
	*/
	intermediateRes := make([][]frontend.Variable, len(b.Nodes))
	for level := range b.Nodes {
		intermediateRes[level] = make([]frontend.Variable, len(b.Nodes[level]))
	}

	/*
		Then, store the initial values in the level entries of the vector
	*/
	inputCursor := 0

	for i := range b.Nodes[0] {
		switch op := b.Nodes[0][i].Operator.(type) {
		case Constant:
			intermediateRes[0][i] = op.Val
		case Variable:
			intermediateRes[0][i] = inputs[inputCursor]
			inputCursor++
		}
	}

	/*
		Then computes the levels one by one
	*/
	for level := 1; level < len(b.Nodes); level++ {
		for pos, node := range b.Nodes[level] {
			/*
				Collect the inputs of the current node from the intermediateRes
			*/
			nodeInputs := make([]frontend.Variable, len(node.Children))
			for i, childID := range node.Children {
				nodeInputs[i] = intermediateRes[childID.level()][childID.posInLevel()]
			}

			/*
				Run the evaluation
			*/
			res := node.Operator.GnarkEval(api, nodeInputs)

			/*
				Registers the result in the intermediate results
			*/
			intermediateRes[level][pos] = res
		}
	}

	// Deep-copy the result from the last level (which assumedly contains only one node)
	if len(intermediateRes[len(intermediateRes)-1]) > 1 {
		panic("multiple heads")
	}
	return intermediateRes[len(b.Nodes)-1][0]

}

// CountNodes returns the node count of the expression
func (b *ExpressionBoard) CountNodes() int {
	res := 0
	for i := 0; i < len(b.Nodes); i++ {
		res += len(b.Nodes[i])
	}
	return res
}

func (b *ExpressionBoard) CountNodesFilterConstants() int {
	res := 0
	for i := 0; i < len(b.Nodes); i++ {
		for j := 0; j < len(b.Nodes[i]); j++ {
			if _, ok := b.Nodes[i][j].Operator.(Constant); !ok {
				res++
			}
		}
	}
	return res
}

func areAllConstants(inp []sv.SmartVector) bool {
	for _, v := range inp {
		// must be sv.Constant or sv.ConstantExt
		switch v.(type) {
		case *sv.Constant:
		default:
			return false
		}
	}
	return true
}
