package gkr

import (
	"fmt"
	"maps"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

type (
	Variable   int    // Variable represents a value in a GKR circuit.
	Identifier uint64 // Identifier is a stable, external name for an input or output variable.
)

// GateAPI is a limited version of frontend.API,
// allowing ring arithmetic operations
type GateAPI interface {
	// ---------------------------------------------------------------------------------------------
	// Arithmetic

	// Add returns res = i1+i2+...in
	Add(i1, i2 Variable, in ...Variable) Variable

	// MulAcc sets and return a = a + (b*c).
	//
	// ! The method may mutate a without allocating a new result. If the input
	// is used elsewhere, then first initialize new variable, for example by
	// doing:
	//
	//     acopy := api.Mul(a, 1)
	//     acopy = api.MulAcc(acopy, b, c)
	//
	// ! But it may not modify a, always use MulAcc(...) result for correctness.
	MulAcc(a, b, c Variable) Variable

	// Neg returns -i
	Neg(i1 Variable) Variable

	// Sub returns res = i1 - i2 - ...in
	Sub(i1, i2 Variable, in ...Variable) Variable

	// Mul returns res = i1 * i2 * ... in
	Mul(i1, i2 Variable, in ...Variable) Variable

	// Const introduces a constant.
	Const(v field.Element) Variable
}

// GateFunction is a function that evaluates a polynomial over its inputs
// using the given GateAPI.
// It is used to define custom gates in GKR circuits.
type GateFunction func(GateAPI, ...Variable) Variable

type API struct {
	circuit   rawCircuit
	positions map[Identifier]Variable
}

// Gate adds the given gate with the given inputs and returns its output wire.
func (api *API) Gate(gate GateFunction, inputs ...Variable) Variable {
	ins := make([]int, len(inputs))
	for i, in := range inputs {
		ins[i] = int(in)
	}
	api.circuit = append(api.circuit, rawWire{Gate: gate, Inputs: ins})
	return Variable(len(api.circuit) - 1)
}

func (api *API) gate2PlusIn(gate GateFunction, in1, in2 Variable, in ...Variable) Variable {
	inCombined := make([]Variable, 2+len(in))
	inCombined[0] = in1
	inCombined[1] = in2
	for i := range in {
		inCombined[i+2] = in[i]
	}
	return api.Gate(gate, inCombined...)
}

func (api *API) Add(i1, i2 Variable) Variable {
	return api.gate2PlusIn(Add2, i1, i2)
}

func (api *API) Neg(i1 Variable) Variable {
	return api.Gate(Neg, i1)
}

func (api *API) Sub(i1, i2 Variable) Variable {
	return api.gate2PlusIn(Sub2, i1, i2)
}

func (api *API) Mul(i1, i2 Variable) Variable {
	return api.gate2PlusIn(Mul2, i1, i2)
}

// newID binds id to v. Internal wires are not bound.
func (api *API) newID(v Variable, id Identifier) {
	if bound, ok := api.positions[id]; ok {
		panic(fmt.Sprintf("gkr: identifier %d already bound to variable %d", id, bound))
	}
	if api.positions == nil {
		api.positions = make(map[Identifier]Variable)
	}
	api.positions[id] = v
}

// Export explicitly designates a wire as output.
// Wires that are not used as input to another are considered output by default.
func (api *API) Export(v Variable, id Identifier) {
	api.newID(v, id)
	api.circuit[v].Exported = true
}

// NewInput creates a new input variable.
func (api *API) NewInput(id Identifier) Variable {
	v := Variable(len(api.circuit))
	api.newID(v, id)
	api.circuit = append(api.circuit, rawWire{})
	return v
}

// Compile traces every gate function into bytecode, determines its degree, and
// derives the proving schedule. It panics on a malformed circuit.
func (api *API) Compile() *Compiled {
	circuit := make(Circuit, len(api.circuit))
	for i, w := range api.circuit {
		circuit[i].Inputs = w.Inputs
		circuit[i].Exported = w.Exported
		if w.IsInput() {
			continue
		}
		if w.Gate == nil {
			panic(fmt.Sprintf("gkr: wire %d has inputs but no gate", i))
		}
		gate, err := CompileGateFunction(w.Gate, len(w.Inputs))
		if err != nil {
			panic(fmt.Sprintf("gkr: wire %d: %v", i, err))
		}
		circuit[i].Gate = gate
	}

	if len(circuit.Inputs()) == len(circuit) {
		panic("gkr: circuit has no non-input wires")
	}

	schedule, err := DefaultProvingSchedule(circuit)
	if err != nil {
		panic(fmt.Sprintf("gkr: %v", err))
	}

	return &Compiled{
		circuit:   circuit,
		schedule:  schedule,
		positions: maps.Clone(api.positions),
	}
}

// Compiled is a circuit ready to be proven or verified. It is the unit that
// Serialize and Deserialize round-trip.
type Compiled struct {
	circuit   Circuit
	schedule  ProvingSchedule
	positions map[Identifier]Variable
}

func (c *Compiled) Serialize() []byte {
	return nil
}

func (c *Compiled) Deserialize(data []byte) error {

}

type Assignment map[Identifier][]field.Ext
