package wiop

import (
	"fmt"

	field "github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// opCode identifies a single VM instruction.
type opCode uint8

const (
	// opLeaf loads the evaluated result of a leaf expression into a slot.
	// Encoding: [opLeaf, dst, leafIdx]
	opLeaf opCode = iota
	// opAdd writes slots[src1] + slots[src2] into slots[dst].
	// Encoding: [opAdd, dst, src1, src2]
	opAdd
	// opSub writes slots[src1] - slots[src2] into slots[dst].
	// Encoding: [opSub, dst, src1, src2]
	opSub
	// opMul writes slots[src1] * slots[src2] into slots[dst].
	// Encoding: [opMul, dst, src1, src2]
	opMul
	// opDiv writes slots[src1] / slots[src2] into slots[dst].
	// Encoding: [opDiv, dst, src1, src2]
	opDiv
	// opDouble writes 2 * slots[src] into slots[dst].
	// Encoding: [opDouble, dst, src]
	opDouble
	// opSquare writes slots[src]² into slots[dst].
	// Encoding: [opSquare, dst, src]
	opSquare
	// opNegate writes -slots[src] into slots[dst].
	// Encoding: [opNegate, dst, src]
	opNegate
	// opInverse writes 1/slots[src] into slots[dst].
	// Encoding: [opInverse, dst, src]
	opInverse
)

// compiledProgram is the bytecode-compiled form of an [ArithmeticOperation]
// subtree. It is computed once and cached as a private field on the root node.
//
// The compilation preserves three key optimisations from the original symbolic
// package compiler:
//  1. Common-subexpression elimination — shared Expression pointers are
//     evaluated once and their result reused.
//  2. Register allocation with liveness analysis — memory slots are reclaimed
//     as soon as the last instruction that needs them completes.
//  3. Base/extension-field dispatch — each slot is statically typed (base or
//     ext) so the VM uses the cheapest arithmetic available.
type compiledProgram struct {
	// leaves holds the non-ArithmeticOperation expressions in the order they
	// are first encountered during compilation. At runtime, each leaf's
	// EvaluateVector result is loaded into the slot emitted by its opLeaf
	// instruction.
	leaves []Expression
	// bytecode is the flat instruction stream.
	bytecode []int
	// slotTypes[i] == true means slot i holds base-field values.
	// Determined statically using Expression.IsExtension().
	slotTypes []bool
	numSlots  int
	// resultSlot is the slot that holds the final result after executing all
	// instructions.
	resultSlot int
}

// compileExpr compiles the expression subtree rooted at root into a
// compiledProgram. It must only be called once per root; callers use
// sync.Once to guarantee this.
func compileExpr(root *ArithmeticOperation) *compiledProgram {
	// -----------------------------------------------------------------------
	// 1. Topological sort — DFS post-order, deduplicated by Expression pointer.
	//    Children always appear before the nodes that use them.
	// -----------------------------------------------------------------------
	var order []Expression        // post-order sequence
	index := map[Expression]int{} // expr → position in order

	var visit func(e Expression)
	visit = func(e Expression) {
		if _, seen := index[e]; seen {
			return
		}
		if op, ok := e.(*ArithmeticOperation); ok {
			for _, child := range op.Operands {
				visit(child)
			}
		}
		index[e] = len(order)
		order = append(order, e)
	}
	visit(root)

	n := len(order)

	// -----------------------------------------------------------------------
	// 2. Collect leaves (any Expression that is not an ArithmeticOperation).
	// -----------------------------------------------------------------------
	var leaves []Expression
	leafIdx := make(map[int]int) // order-index → index in leaves
	for i, e := range order {
		if _, ok := e.(*ArithmeticOperation); !ok {
			leafIdx[i] = len(leaves)
			leaves = append(leaves, e)
		}
	}

	// -----------------------------------------------------------------------
	// 3. Static type analysis.
	//    nodeIsBase[i] == true iff order[i] evaluates to base-field values.
	// -----------------------------------------------------------------------
	nodeIsBase := make([]bool, n)
	for i, e := range order {
		nodeIsBase[i] = !e.IsExtension()
	}

	// -----------------------------------------------------------------------
	// 4. Liveness analysis.
	//    lastUse[i] is the index of the latest instruction that reads slot i.
	//    The result node is considered used at position n (after all nodes).
	// -----------------------------------------------------------------------
	lastUse := make([]int, n)
	for i := range lastUse {
		lastUse[i] = -1
	}
	lastUse[n-1] = n

	for i, e := range order {
		if op, ok := e.(*ArithmeticOperation); ok {
			for _, child := range op.Operands {
				ci := index[child]
				if i > lastUse[ci] {
					lastUse[ci] = i
				}
			}
		}
	}

	// -----------------------------------------------------------------------
	// 5. Register allocation — separate free-lists for base and ext slots so
	//    that a reused slot always has the correct pre-allocated backing type.
	// -----------------------------------------------------------------------
	slots := make([]int, n)
	freeBase := []int{}
	freeExt := []int{}
	var slotTypes []bool
	nextSlot := 0

	getSlot := func(isBase bool) int {
		free := &freeExt
		if isBase {
			free = &freeBase
		}
		if len(*free) > 0 {
			s := (*free)[len(*free)-1]
			*free = (*free)[:len(*free)-1]
			return s
		}
		s := nextSlot
		nextSlot++
		slotTypes = append(slotTypes, isBase)
		return s
	}

	// -----------------------------------------------------------------------
	// 6. Instruction emission.
	// -----------------------------------------------------------------------
	bytecode := make([]int, 0, n*4)

	for i, e := range order {
		dst := getSlot(nodeIsBase[i])
		slots[i] = dst

		if op, ok := e.(*ArithmeticOperation); ok {
			switch op.Operator {
			case ArithmeticOperatorAdd:
				s1, s2 := slots[index[op.Operands[0]]], slots[index[op.Operands[1]]]
				bytecode = append(bytecode, int(opAdd), dst, s1, s2)
			case ArithmeticOperatorSub:
				s1, s2 := slots[index[op.Operands[0]]], slots[index[op.Operands[1]]]
				bytecode = append(bytecode, int(opSub), dst, s1, s2)
			case ArithmeticOperatorMul:
				s1, s2 := slots[index[op.Operands[0]]], slots[index[op.Operands[1]]]
				bytecode = append(bytecode, int(opMul), dst, s1, s2)
			case ArithmeticOperatorDiv:
				s1, s2 := slots[index[op.Operands[0]]], slots[index[op.Operands[1]]]
				bytecode = append(bytecode, int(opDiv), dst, s1, s2)
			case ArithmeticOperatorDouble:
				s := slots[index[op.Operands[0]]]
				bytecode = append(bytecode, int(opDouble), dst, s)
			case ArithmeticOperatorSquare:
				s := slots[index[op.Operands[0]]]
				bytecode = append(bytecode, int(opSquare), dst, s)
			case ArithmeticOperatorNegate:
				s := slots[index[op.Operands[0]]]
				bytecode = append(bytecode, int(opNegate), dst, s)
			case ArithmeticOperatorInverse:
				s := slots[index[op.Operands[0]]]
				bytecode = append(bytecode, int(opInverse), dst, s)
			}
		} else {
			bytecode = append(bytecode, int(opLeaf), dst, leafIdx[i])
		}

		// Free dead children's slots back into the appropriate free-list.
		if op, ok := e.(*ArithmeticOperation); ok {
			freed := map[int]bool{}
			for _, child := range op.Operands {
				ci := index[child]
				if lastUse[ci] == i && !freed[slots[ci]] {
					freed[slots[ci]] = true
					if nodeIsBase[ci] {
						freeBase = append(freeBase, slots[ci])
					} else {
						freeExt = append(freeExt, slots[ci])
					}
				}
			}
		}
	}

	return &compiledProgram{
		leaves:     leaves,
		bytecode:   bytecode,
		slotTypes:  slotTypes,
		numSlots:   nextSlot,
		resultSlot: slots[n-1],
	}
}

// evaluateVector runs the compiled program against the given runtime and
// returns the resulting [field.Vec].
//
// Assumptions about ConcreteVector:
//   - A leaf's ConcreteVector.Plain is a non-empty slice; Plain[0] is the
//     column data this compiler operates on. This assumption should be revisited
//     once the Runtime / ConcreteVector contract is finalised.
func (p *compiledProgram) evaluateVector(rt Runtime) field.Vec {
	// ------------------------------------------------------------------
	// Evaluate all leaves up-front to determine the vector length and to
	// avoid re-evaluating leaves inside the main execution loop.
	// ------------------------------------------------------------------
	leafVecs := make([]field.Vec, len(p.leaves))
	n := 0
	for i, leaf := range p.leaves {
		cv := leaf.EvaluateVector(rt)
		leafVecs[i] = cv.Plain
		leafLen := leafVecs[i].Len()
		if i == 0 {
			n = leafLen
		} else if leafLen != n {
			//nolint
			panic(fmt.Sprintf(
				"wiop: compiler: leaf %d has length %d but leaf 0 has length %d; all leaves must have the same size",
				i, leafLen, n,
			))
		}
	}

	// ------------------------------------------------------------------
	// Allocate backing memory for every slot up-front.
	// The type (base vs ext) is known statically from compilation.
	// ------------------------------------------------------------------
	slots := make([]field.Vec, p.numSlots)
	for i, isBase := range p.slotTypes {
		if isBase {
			slots[i] = field.VecFromBase(make([]field.Element, n))
		} else {
			slots[i] = field.VecFromExt(make([]field.Ext, n))
		}
	}

	// ------------------------------------------------------------------
	// Execute the bytecode.
	// ------------------------------------------------------------------
	bc := p.bytecode
	for pc := 0; pc < len(bc); {
		op := opCode(bc[pc])
		dst := bc[pc+1]
		pc += 2

		switch op {
		case opLeaf:
			li := bc[pc]
			pc++
			lv := leafVecs[li]
			if lv.IsBase() {
				copy(slots[dst].AsBase(), lv.AsBase())
			} else {
				copy(slots[dst].AsExt(), lv.AsExt())
			}

		case opAdd:
			s1, s2 := bc[pc], bc[pc+1]
			pc += 2
			field.VecAddInto(slots[dst], slots[s1], slots[s2])

		case opSub:
			s1, s2 := bc[pc], bc[pc+1]
			pc += 2
			field.VecSubInto(slots[dst], slots[s1], slots[s2])

		case opMul:
			s1, s2 := bc[pc], bc[pc+1]
			pc += 2
			field.VecMulInto(slots[dst], slots[s1], slots[s2])

		case opDiv:
			// dst = src1 / src2 = src1 * src2⁻¹
			// The inverse is computed into a temporary with the same field
			// type as src2 to satisfy VecBatchInvInto's same-type requirement.
			s1, s2 := bc[pc], bc[pc+1]
			pc += 2
			var inv2 field.Vec
			if slots[s2].IsBase() {
				inv2 = field.VecFromBase(make([]field.Element, n))
			} else {
				inv2 = field.VecFromExt(make([]field.Ext, n))
			}
			field.VecBatchInvInto(inv2, slots[s2])
			field.VecMulInto(slots[dst], slots[s1], inv2)

		case opDouble:
			s := bc[pc]
			pc++
			field.VecAddInto(slots[dst], slots[s], slots[s])

		case opSquare:
			s := bc[pc]
			pc++
			field.VecMulInto(slots[dst], slots[s], slots[s])

		case opNegate:
			s := bc[pc]
			pc++
			field.VecNegInto(slots[dst], slots[s])

		case opInverse:
			s := bc[pc]
			pc++
			field.VecBatchInvInto(slots[dst], slots[s])
		}
	}

	return slots[p.resultSlot]
}
