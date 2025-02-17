package simplify

import (
	"math/bits"
	"sync"

	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

// Returns the cost of evaluating the expression in number of multiplications
type costStats struct {
	NumMul int
	NumAdd int
}

// add a cost stats into the current cost stats.
func (s *costStats) add(cost costStats) {
	s.NumAdd += cost.NumAdd
	s.NumMul += cost.NumMul
}

// Returns the cost stats of a boarded expression
func evaluateCostStat(expr *sym.Expression) (s costStats) {
	board := expr.Board()
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 1; i < len(board.Nodes); i++ {
		wg.Add(1)
		go func(nodes []sym.Node) {
			defer wg.Done()
			s_ := evaluateNodeCosts(nodes...)
			mu.Lock()
			s.add(s_)
			mu.Unlock()
		}(board.Nodes[i])
	}
	wg.Wait()

	return s
}

// Returns the cost stats of a node
func evaluateNodeCosts(nodes ...sym.Node) (s costStats) {
	for _, node := range nodes {
		switch op := node.Operator.(type) {
		case sym.Product:
			for _, e := range op.Exponents {
				if e == 0 {
					continue
				}

				// Rule of thumb, that exponent and look at the
				// bits. Observe "a" the number of non-zero bits and
				// "b" the position of the leading non-zero bit. The
				// cost is a + b - 1 if we use a double and add
				// algorithm to perform the exponentiation and its
				// accumulation. It is assumed that e is non-zero

				e_ := uint64(e)
				a := bits.OnesCount64(e_)
				b := 64 - bits.LeadingZeros64(e_)
				s.NumMul += a + b - 1
			}
			// Compensate for the fact that that we don't need to multiply the
			// initial term into the result
			s.NumMul--
		case sym.LinComb:
			for _, e := range op.Coeffs {
				if e == 0 {
					continue
				}

				// The technique here is to to use addition for small
				// coefficients because this is a faster than multiplying
				// in a field element. We do that up to 2 and -2. The
				// second assumption is that addition and substraction
				// are equivalent in term of runtime. Otherwise, this is
				// done with a multiplication and an addition.

				// Reduce to a positive number
				if e < 0 {
					e = -e
				}

				if e <= 2 {
					s.NumAdd += e
				} else {
					s.NumAdd += 1
					s.NumMul += 2
				}
			}
			// Compensate for the fact that we don't need to add the initial
			// term into the result
			s.NumAdd--
		case sym.PolyEval:
			s.NumAdd += len(node.Children) - 1
			s.NumMul += len(node.Children) - 1
		}
	}

	return s
}
