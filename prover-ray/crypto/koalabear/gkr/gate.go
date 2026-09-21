package gkr

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// Ported from gnark's internal/gkr/gkrcore/gate.go.

// GateOp represents an arithmetic operation in a compiled gate.
type GateOp uint8

const (
	OpAdd    GateOp = iota // result = src1 + src2 + ... (variadic)
	OpSub                  // result = src1 - src2 - ...
	OpMul                  // result = src1 * src2 * ...
	OpNeg                  // result = -src1
	OpMulAcc               // result = src1 + (src2 * src3)
)

// String returns a human-readable representation of the operation.
func (op GateOp) String() string {
	switch op {
	case OpAdd:
		return "add"
	case OpSub:
		return "sub"
	case OpMul:
		return "mul"
	case OpNeg:
		return "neg"
	case OpMulAcc:
		return "mulacc"
	default:
		return "unknown"
	}
}

// GateInstruction represents a single operation in a compiled gate.
// Each instruction produces a new variable (no explicit dst field).
// Index space layout:
//   - [0, nbConsts): constant values (from GateBytecode.Constants)
//   - [nbConsts, nbConsts+nbInputs): gate inputs
//   - [nbConsts+nbInputs, ...): instruction results
type GateInstruction struct {
	Op     GateOp
	Inputs []uint16 // indices into the unified value space
}

// GateBytecode represents a gate executable compiled into a sequence of instructions.
type GateBytecode struct {
	Instructions []GateInstruction
	Constants    []*big.Int
}

// IdentityBytecode returns the compiled form of the identity gate (x → x).
// A GateBytecode with no instructions returns its sole input directly.
func IdentityBytecode() GateBytecode { return GateBytecode{} }

// NbConstants returns the number of constants in the gate.
func (g *GateBytecode) NbConstants() int { return len(g.Constants) }

// EstimateDegree returns an upper bound on the degree of the gate.
func (g *GateBytecode) EstimateDegree(nbIn int) int {
	frameSize := len(g.Constants) + nbIn
	deg := make([]int, frameSize+len(g.Instructions))
	for i := range nbIn {
		deg[i+len(g.Constants)] = 1
	}
	for i, inst := range g.Instructions {
		var curr int
		switch inst.Op {
		case OpAdd, OpSub, OpNeg:
			for _, in := range inst.Inputs {
				curr = max(curr, deg[in])
			}
		case OpMul:
			for _, in := range inst.Inputs {
				curr += deg[in]
			}
		case OpMulAcc: // a + b*c
			curr = max(deg[inst.Inputs[0]], deg[inst.Inputs[1]]+deg[inst.Inputs[2]])
		default:
			panic("unknown operation")
		}
		deg[frameSize+i] = curr
	}
	return deg[len(deg)-1]
}

// gateCompiler is an implementation of GateAPI that records operations instead of
// executing them. During compilation, temporary indices are used:
//   - Constants: high indices (starting at constMarker)
//   - Inputs: 0..nbInputs-1
//   - Results: nbInputs onwards
//
// After compilation, indices are remapped to: constants, inputs, results.
type gateCompiler struct {
	instructions  []GateInstruction
	constants     []*big.Int
	constantIndex map[string]Variable // constant value → temp index
	nbInputs      int
}

const constMarker = 0x8000

func (gc *gateCompiler) addInstruction(op GateOp, inputs ...Variable) Variable {
	ins := make([]uint16, len(inputs))
	for i := range ins {
		ins[i] = uint16(inputs[i])
	}

	res := Variable(len(gc.instructions) + gc.nbInputs)
	gc.instructions = append(gc.instructions, GateInstruction{Op: op, Inputs: ins})
	return res
}

func (gc *gateCompiler) addInstruction2Plus(op GateOp, i1, i2 Variable, in ...Variable) Variable {
	ins := make([]Variable, len(in)+2)
	ins[0] = i1
	ins[1] = i2
	copy(ins[2:], in)
	return gc.addInstruction(op, ins...)
}

// Const introduces a constant into the gate's value space.
func (gc *gateCompiler) Const(v field.Element) Variable {
	var val big.Int
	v.BigInt(&val)

	key := val.String()
	if i, ok := gc.constantIndex[key]; ok {
		return i
	}

	i := Variable(len(gc.constants)) | constMarker
	gc.constants = append(gc.constants, &val)
	gc.constantIndex[key] = i
	return i
}

func (gc *gateCompiler) Add(i1, i2 Variable, in ...Variable) Variable {
	return gc.addInstruction2Plus(OpAdd, i1, i2, in...)
}

func (gc *gateCompiler) MulAcc(a, b, c Variable) Variable {
	return gc.addInstruction(OpMulAcc, a, b, c)
}

func (gc *gateCompiler) Neg(i1 Variable) Variable {
	return gc.addInstruction(OpNeg, i1)
}

func (gc *gateCompiler) Sub(i1, i2 Variable, in ...Variable) Variable {
	return gc.addInstruction2Plus(OpSub, i1, i2, in...)
}

func (gc *gateCompiler) Mul(i1, i2 Variable, in ...Variable) Variable {
	return gc.addInstruction2Plus(OpMul, i1, i2, in...)
}

// remapIndices transforms temporary indices to the final layout: constants, inputs, results.
func (gc *gateCompiler) remapIndices() {
	nbConsts := uint16(len(gc.constants))
	for i := range gc.instructions {
		for j := range gc.instructions[i].Inputs {
			if gc.instructions[i].Inputs[j]&constMarker != 0 {
				gc.instructions[i].Inputs[j] &= ^uint16(constMarker)
			} else {
				gc.instructions[i].Inputs[j] += nbConsts
			}
		}
	}
}

// CompileGateFunction traces f into bytecode and determines its degree.
func CompileGateFunction(f GateFunction, nbInputs int) (Gate, error) {
	compiler := gateCompiler{
		constantIndex: make(map[string]Variable),
		nbInputs:      nbInputs,
	}

	inputs := make([]Variable, nbInputs)
	for i := range inputs {
		inputs[i] = Variable(i)
	}

	outVar := f(&compiler, inputs...)

	if len(compiler.instructions) == 0 {
		// No operations recorded, but not all is lost yet.
		// If the output simply mirrors the last input, we can still represent
		// it in bytecode, as the evaluator returns the last stack frame element.
		if int(outVar) == len(compiler.constants)+nbInputs-1 {
			return Gate{NbIn: nbInputs, Degree: 1}, nil
		}
		return Gate{}, errors.New("only non-trivial or last-reflective gate functions supported")
	}

	// All instructions after the output are no-ops. Prune them and the corresponding variables.
	// Henceforth, we guarantee that the variable with the highest index is the gate output.
	lastEffectiveInstructionI := int(outVar) - compiler.nbInputs
	compiler.instructions = compiler.instructions[:lastEffectiveInstructionI+1]

	compiler.remapIndices()

	bytecode := GateBytecode{
		Instructions: compiler.instructions,
		Constants:    compiler.constants,
	}

	tester := gateTester{mod: field.Modulus()}
	tester.setGate(bytecode, nbInputs)

	degree := len(tester.fitPoly(bytecode.EstimateDegree(nbInputs))) - 1
	if degree == -1 {
		return Gate{}, errors.New("cannot find degree for gate")
	}

	return Gate{Evaluate: bytecode, NbIn: nbInputs, Degree: degree}, nil
}

type gateTester struct {
	mod  *big.Int
	gate GateBytecode
	vars []*big.Int
	nbIn int
}

func (t *gateTester) setGate(g GateBytecode, nbIn int) {
	t.gate = g
	t.vars = make([]*big.Int, g.NbConstants()+nbIn+len(g.Instructions))
	t.nbIn = nbIn
	copy(t.vars, g.Constants)
}

func (t *gateTester) isZero(a *big.Int) bool {
	return new(big.Int).Mod(a, t.mod).BitLen() == 0
}

func (t *gateTester) equal(a, b *big.Int) bool { return a.Cmp(b) == 0 }

func (t *gateTester) add(a, b *big.Int) *big.Int {
	res := new(big.Int).Add(a, b)
	return res.Mod(res, t.mod)
}

func (t *gateTester) sub(a, b *big.Int) *big.Int {
	res := new(big.Int).Sub(a, b)
	return res.Mod(res, t.mod)
}

func (t *gateTester) mul(a, b *big.Int) *big.Int {
	res := new(big.Int).Mul(a, b)
	return res.Mod(res, t.mod)
}

func (t *gateTester) neg(a *big.Int) *big.Int {
	res := new(big.Int).Neg(a)
	return res.Mod(res, t.mod)
}

func (t *gateTester) inverse(a *big.Int) *big.Int {
	return new(big.Int).ModInverse(a, t.mod)
}

func (t *gateTester) div(a, b *big.Int) *big.Int {
	res := new(big.Int).ModInverse(b, t.mod)
	return res.Mul(a, res).Mod(res, t.mod)
}

func (t *gateTester) randomElement() *big.Int {
	res, err := rand.Int(rand.Reader, t.mod)
	if err != nil {
		panic(err)
	}
	return res
}

func (t *gateTester) randomElements(n int) []*big.Int {
	res := make([]*big.Int, n)
	for i := range res {
		res[i] = t.randomElement()
	}
	return res
}

func (t *gateTester) evalPoly(p []*big.Int, x *big.Int) *big.Int {
	res := p[len(p)-1]
	for i := len(p) - 2; i >= 0; i-- {
		res = t.mul(res, x)
		res = t.add(res, p[i])
	}
	return res
}

// evaluate executes the gate bytecode with the given inputs.
func (t *gateTester) evaluate(inputs ...*big.Int) *big.Int {
	frameSize := t.gate.NbConstants()
	copy(t.vars[frameSize:], inputs)
	frameSize += len(inputs)

	for _, inst := range t.gate.Instructions {
		dst := t.vars[frameSize]
		if dst == nil {
			dst = new(big.Int)
			t.vars[frameSize] = dst
		}
		switch inst.Op {
		case OpAdd:
			dst.Set(t.vars[inst.Inputs[0]])
			for _, i := range inst.Inputs[1:] {
				dst.Add(dst, t.vars[i])
			}
		case OpSub:
			dst.Set(t.vars[inst.Inputs[0]])
			for _, i := range inst.Inputs[1:] {
				dst.Sub(dst, t.vars[i])
			}
		case OpMul:
			dst.Set(t.vars[inst.Inputs[0]])
			for _, i := range inst.Inputs[1:] {
				dst.Mul(dst, t.vars[i])
			}
		case OpNeg:
			dst.Neg(t.vars[inst.Inputs[0]])
		case OpMulAcc: // a + b*c
			dst.Mul(t.vars[inst.Inputs[1]], t.vars[inst.Inputs[2]])
			dst.Add(dst, t.vars[inst.Inputs[0]])
		default:
			panic("unknown operation")
		}
		dst.Mod(dst, t.mod)
		frameSize++
	}

	return new(big.Int).Set(t.vars[frameSize-1])
}

// fitPoly tries to fit a polynomial of degree no more than maxDegree to the gate.
// It returns the polynomial if successful, nil otherwise.
func (t *gateTester) fitPoly(maxDegree int) []*big.Int {
	// turn f univariate by defining p(x) as f(x, rx, ..., sx)
	// where r, s, ... are random constants
	fIn := make([]*big.Int, t.nbIn)
	consts := t.randomElements(t.nbIn - 1)

	p := make([]*big.Int, maxDegree+1)

	x := t.randomElements(maxDegree + 1)
	for i := range x {
		fIn[0] = x[i]
		for j := range consts {
			fIn[j+1] = t.mul(x[i], consts[j])
		}
		p[i] = t.evaluate(fIn...)
	}

	// obtain p's coefficients
	p, err := t.interpolate(x, p)
	if err != nil {
		panic(err)
	}

	// check if p is equal to f. This not being the case means that f is of a degree higher than maxDegree
	fIn[0] = t.randomElement()
	for i := range consts {
		fIn[i+1] = t.mul(fIn[0], consts[i])
	}
	if !t.equal(t.evalPoly(p, fIn[0]), t.evaluate(fIn...)) {
		return nil
	}

	// trim p
	lastNonZero := len(p) - 1
	for lastNonZero >= 0 && t.isZero(p[lastNonZero]) {
		lastNonZero--
	}
	return p[:lastNonZero+1]
}

// interpolate fits a polynomial of degree len(X) - 1 = len(Y) - 1 to the points (X[i], Y[i]).
// Note that the runtime is O(len(X)³).
func (t *gateTester) interpolate(X, Y []*big.Int) ([]*big.Int, error) {
	if len(X) != len(Y) {
		return nil, errors.New("same length expected for X and Y")
	}

	one := big.NewInt(1)

	// solve the system of equations by Gaussian elimination
	augmentedRows := make([][]*big.Int, len(X)) // the last column is the Y values
	for i := range augmentedRows {
		augmentedRows[i] = make([]*big.Int, len(X)+1)
		augmentedRows[i][0] = one
		augmentedRows[i][1] = X[i]
		for j := 2; j < len(augmentedRows[i])-1; j++ {
			augmentedRows[i][j] = t.mul(augmentedRows[i][j-1], X[i])
		}
		augmentedRows[i][len(augmentedRows[i])-1] = Y[i]
	}

	// make the upper triangle
	for i := range len(augmentedRows) - 1 {
		// use row i to eliminate the ith element in all rows below
		if t.isZero(augmentedRows[i][i]) {
			return nil, errors.New("singular matrix")
		}
		negInv := t.neg(t.inverse(augmentedRows[i][i]))
		for j := i + 1; j < len(augmentedRows); j++ {
			c := t.mul(augmentedRows[j][i], negInv)
			for k := i + 1; k < len(augmentedRows[i]); k++ {
				augmentedRows[j][k] = t.add(augmentedRows[j][k], t.mul(augmentedRows[i][k], c))
			}
		}
	}

	// back substitution
	res := make([]*big.Int, len(X))
	for i := len(augmentedRows) - 1; i >= 0; i-- {
		res[i] = augmentedRows[i][len(augmentedRows[i])-1]
		for j := i + 1; j < len(augmentedRows[i])-1; j++ {
			res[i] = t.sub(res[i], t.mul(res[j], augmentedRows[i][j]))
		}
		res[i] = t.div(res[i], augmentedRows[i][i])
	}

	return res, nil
}
