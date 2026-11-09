package wiop

// WalkLeaves calls visit on the leaves of expr in depth-first, left-to-right
// order, descending into each distinct [ArithmeticOperation] once. Expressions
// are DAGs (equal subexpressions share a pointer), so a walk that re-descended
// into a shared node would visit it once per path to it, which is exponential
// in the depth.
//
// visited records the operation nodes already descended into. Pass nil for a
// walk of expr alone, or the same map to several calls to skip the nodes an
// earlier call already walked. A leaf reachable only through a visited node is
// not reported again, so across the calls the order of first occurrences is
// that of the full tree walks. A leaf may be reported more than once when it is
// reached through several distinct nodes.
func WalkLeaves(expr Expression, visited map[*ArithmeticOperation]struct{}, visit func(Expression)) {
	if visited == nil {
		visited = make(map[*ArithmeticOperation]struct{})
	}
	var walk func(Expression)
	walk = func(e Expression) {
		op, ok := e.(*ArithmeticOperation)
		if !ok {
			visit(e)
			return
		}
		if _, done := visited[op]; done {
			return
		}
		visited[op] = struct{}{}
		for _, o := range op.Operands {
			walk(o)
		}
	}
	walk(expr)
}
