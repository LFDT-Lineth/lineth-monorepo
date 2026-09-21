package gkr

// Ported from gnark's internal/gkr/gkrcore.

type (
	// Gate is a low-degree multivariate polynomial, compiled to bytecode.
	Gate struct {
		Evaluate GateBytecode
		NbIn     int // number of inputs
		Degree   int // total degree of the polynomial
	}

	// Wire is a single wire of a compiled circuit. Its Inputs are the indices of
	// the wires feeding its gate; an empty Inputs marks a circuit input.
	Wire struct {
		Gate     Gate
		Inputs   []int
		Exported bool
	}

	// Circuit is a compiled GKR circuit. A wire's inputs always have lower indices
	// than the wire itself, so the slice order is a topological order.
	Circuit []Wire

	// rawWire is a wire of a circuit under construction, holding the gate function
	// before it is traced into bytecode.
	rawWire struct {
		Gate     GateFunction
		Inputs   []int
		Exported bool
	}

	// rawCircuit is a circuit under construction.
	rawCircuit []rawWire
)

// IsInput returns whether the wire is an input wire.
func (w Wire) IsInput() bool { return len(w.Inputs) == 0 }

// IsInput returns whether the wire at wireI is an input wire.
func (c Circuit) IsInput(wireI int) bool { return c[wireI].IsInput() }

// IsInput returns whether the wire is an input wire.
func (w rawWire) IsInput() bool { return len(w.Inputs) == 0 }

func (c Circuit) maxGateDegree() int {
	res := 1
	for i := range c {
		if !c[i].IsInput() {
			res = max(res, c[i].Gate.Degree)
		}
	}
	return res
}

// Inputs returns the list of input wire indices.
func (c Circuit) Inputs() []int {
	res := make([]int, 0, len(c))
	for i := range c {
		if c[i].IsInput() {
			res = append(res, i)
		}
	}
	return res
}

// Outputs returns the list of output wire indices: those not consumed by any
// other wire, plus those explicitly exported.
func (c Circuit) Outputs() []int {
	isOutputTo := make([]bool, len(c))
	for i := range c {
		for _, in := range c[i].Inputs {
			isOutputTo[in] = true
		}
	}
	res := make([]int, 0, len(c))
	for i := range c {
		if !isOutputTo[i] || c[i].Exported {
			res = append(res, i)
		}
	}
	return res
}

// MaxGateNbIn returns the maximum number of inputs of any gate in the circuit.
func (c Circuit) MaxGateNbIn() int {
	res := 0
	for i := range c {
		res = max(res, len(c[i].Inputs))
	}
	return res
}

// some sample gates

// Identity gate: x -> x
func Identity(_ GateAPI, in ...Variable) Variable { return in[0] }

// Add2 gate: (x, y) -> x + y
func Add2(api GateAPI, in ...Variable) Variable { return api.Add(in[0], in[1]) }

// Sub2 gate: (x, y) -> x - y
func Sub2(api GateAPI, in ...Variable) Variable { return api.Sub(in[0], in[1]) }

// Neg gate: x -> -x
func Neg(api GateAPI, in ...Variable) Variable { return api.Neg(in[0]) }

// Mul2 gate: (x, y) -> x * y
func Mul2(api GateAPI, in ...Variable) Variable { return api.Mul(in[0], in[1]) }
