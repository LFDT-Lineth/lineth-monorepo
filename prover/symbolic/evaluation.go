package symbolic

import (
	"fmt"
	"sync"

	"github.com/consensys/gnark/frontend"

	"github.com/consensys/linea-monorepo/prover/maths/common/mempool"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

const (
	// MaxChunkSize is a fine-tuned by hand number. Smaller is slower, larger
	// comsumes for more memory. For testing, it can be overriden even though
	// it will become flakky if we have several tests.
	MaxChunkSize int = 1 << 9
)

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

// Evaluate the board for a batch  of inputs in parallel
func (b *ExpressionBoard) Evaluate(inputs []sv.SmartVector, p ...mempool.MemPool) sv.SmartVector {

	/*
		Find the size of the vector
	*/
	totalSize := 0
	for i, inp := range inputs {
		if totalSize > 0 && totalSize != inp.Len() {
			// Expects that all vector inputs have the same size
			utils.Panic("Mismatch in the size: len(v) %v, totalsize %v, pos %v", inp.Len(), totalSize, i)
		}

		if totalSize == 0 {
			totalSize = inp.Len()
		}
	}

	if totalSize == 0 {
		// Either, there is no input or the inputs are empty. Both
		// cases are panic
		utils.Panic("Either there is no input or the inputs all have size 0")
	}

	if len(p) > 0 && p[0].Size() != MaxChunkSize {
		utils.Panic("the pool should be a pool of vectors of size=%v but it is %v", MaxChunkSize, p[0].Size())
	}

	/*
		The is no vector input iff totalSize is 0
		Thus the condition below catch the two cases where:
			- There is no input vectors (scalars only)
			- The vectors are smaller than the min chunk size
	*/

	if totalSize < MaxChunkSize {
		// never pass the pool here as the pool assumes that all vectors have a
		// size of MaxChunkSize. Thus, it would not work here.
		return b.evaluateSingleThread(inputs).DeepCopy()
	}

	// This is the code-path that is used for benchmarking when the size of the
	// vectors is exactly MaxChunkSize. In production, it will rather use the
	// multi-threaded option. The above condition cannot use the pool because we
	// assume here that the pool has a vector size of exactly MaxChunkSize.
	if totalSize == MaxChunkSize {
		return b.evaluateSingleThread(inputs, p...).DeepCopy()
	}

	if totalSize%MaxChunkSize != 0 {
		utils.Panic("Unsupported : totalSize %v is not divided by the chunk size %v", totalSize, MaxChunkSize)
	}

	numChunks := totalSize / MaxChunkSize
	res := make([]field.Element, totalSize)

	parallel.ExecuteFromChan(numChunks, func(wg *sync.WaitGroup, id *parallel.AtomicCounter) {

		var pool []mempool.MemPool
		if len(p) > 0 {
			if _, ok := p[0].(*mempool.DebuggeableCall); !ok {
				pool = append(pool, mempool.WrapsWithMemCache(p[0]))
			}
		}

		chunkInputs := make([]sv.SmartVector, len(inputs))

		for {
			chunkID, ok := id.Next()
			if !ok {
				break
			}

			var (
				chunkStart = chunkID * MaxChunkSize
				chunkStop  = (chunkID + 1) * MaxChunkSize
			)

			for i, inp := range inputs {
				chunkInputs[i] = inp.SubVector(chunkStart, chunkStop)
				// Sanity-check : the output of SubVector must have the correct length
				if chunkInputs[i].Len() != chunkStop-chunkStart {
					utils.Panic("Subvector failed, subvector should have size %v but size is %v", chunkStop-chunkStart, chunkInputs[i].Len())
				}
			}

			// We don't parallelize evaluations where the inputs are all scalars
			// Therefore the cast is safe.
			chunkRes := b.evaluateSingleThread(chunkInputs, pool...)

			// If chunkRes is all zeroes, then we can skip the copying. As res
			// is already zero. This optimization is particularly useful for the
			// limitless prover where we often deal with very sparse vectors.
			if c, ok := chunkRes.(*sv.Constant); !ok || c.Val() != field.Zero() {
				// No race condition here as each call write to different places
				// of vec.
				chunkRes.WriteInSlice(res[chunkStart:chunkStop])
			}

			wg.Done()
		}

		if len(p) > 0 {
			if sa, ok := pool[0].(*mempool.SliceArena); ok {
				sa.TearDown()
			}
		}
	})

	return sv.NewRegular(res)
}

// evaluateSingleThread evaluates a boarded expression. The inputs can be either
// vector or scalars. The vector's input length should be smaller than a chunk.
func (b *ExpressionBoard) evaluateSingleThread(inputs []sv.SmartVector, p ...mempool.MemPool) sv.SmartVector {

	var (
		length         = inputs[0].Len()
		pool, hasPool  = mempool.ExtractCheckOptionalSoft(length, p...)
		nodeAssignment = b.prepareNodeAssignments(inputs)
	)

	// Then computes the levels one by one
	for level := 1; level < len(nodeAssignment); level++ {
		for pil := range nodeAssignment[level] {

			node := &nodeAssignment[level][pil]
			nodeAssignment.eval(node, pool)
		}
	}

	// Assertion, the last level contains only one node (the final result)
	if len(nodeAssignment[len(nodeAssignment)-1]) > 1 {
		panic("multiple heads")
	}

	resBuf := nodeAssignment[len(b.Nodes)-1][0].Value

	if resBuf == nil {
		panic("resbuf is nil")
	}

	// Deep-copy the last node and put resBuf back in the pool. It's cleanier
	// that way.
	if hasPool {
		if reg, ok := resBuf.(*sv.Pooled); ok {
			resGC := reg.DeepCopy()
			reg.Free(pool)
			resBuf = resGC
		}
	}

	return resBuf
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

// DumpToString is a debug utility which print out the expression in a readable
// format.
func (b *ExpressionBoard) DumpToString() string {
	res := ""
	for i := range b.Nodes {
		res += fmt.Sprintf("LEVEL %v\n", i)
		for _, node := range b.Nodes[i] {
			res += fmt.Sprintf("\t(%T) %++v\n", node.Operator, node)
		}
	}
	return res
}

// CountNodes returns the node count of the expression, without accounting for
// the level zero.
func (b *ExpressionBoard) CountNodes() int {
	res := 0
	for i := 1; i < len(b.Nodes); i++ {
		res += len(b.Nodes[i])
	}
	return res
}
