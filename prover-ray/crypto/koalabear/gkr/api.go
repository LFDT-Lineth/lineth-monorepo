package gkr

import (
	"github.com/consensys/gnark/std/gkrapi/gkr"
)

// Variable represents a value in a GKR circuit.
type Variable int

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
}

// GateFunction is a function that evaluates a polynomial over its inputs
// using the given GateAPI.
// It is used to define custom gates in GKR circuits.
type GateFunction func(GateAPI, ...Variable) Variable

type API struct {
	circuit Circuit
}

// Gate adds the given gate with the given inputs and returns its output wire.
func (api *API) Gate(gate gkr.GateFunction, inputs ...Variable) Variable {
	/*api.circuit = append(api.circuit, gkrcore.RawWire{
		Gate:   gate,
		Inputs: utils.Map(inputs, frontendVarToInt),
	})
	api.assignments = append(api.assignments, nil)
	return Variable(len(api.circuit) - 1)*/
	return -1
}

func (api *API) gate2PlusIn(gate gkr.GateFunction, in1, in2 Variable, in ...Variable) Variable {
	inCombined := make([]Variable, 2+len(in))
	inCombined[0] = in1
	inCombined[1] = in2
	for i := range in {
		inCombined[i+2] = in[i]
	}
	return api.Gate(gate, inCombined...)
}

func (api *API) Add(i1, i2 Variable) Variable {
	return api.gate2PlusIn(gkrcore.Add2, i1, i2)
}

func (api *API) Neg(i1 Variable) Variable {
	return api.Gate(gkrcore.Neg, i1)
}

func (api *API) Sub(i1, i2 Variable) Variable {
	return api.gate2PlusIn(gkrcore.Sub2, i1, i2)
}

func (api *API) Mul(i1, i2 Variable) Variable {
	return api.gate2PlusIn(gkrcore.Mul2, i1, i2)
}

// Export explicitly designates a wire as output.
// Wires that are not used as input to another are considered output by default.
func (api *API) Export(i Variable, name string) {
	for _, v := range in {
		api.circuit[v].Exported = true
	}
}

// NewInput creates a new input variable.
func (api *API) NewInput(name string) Variable {
	i := len(api.circuit)
	api.circuit = append(api.circuit, gkrcore.RawWire{})
	api.assignments = append(api.assignments, nil)
	return gkr.Variable(i)
}

func (api *API) Serialize() []byte {
	return nil
}

func (api *API) Deserialize(data []byte) {

}
