// Package circuit provides circuit (gnark) representations of KoalaBear field elements.
//
// This package contains two main types:
//   - [Var]: A circuit variable over the KoalaBear base field
//   - [Ext]: A circuit variable over the degree-6 extension field
//
// KoalaBear is always emulated in the circuit's field: the gnark circuits only
// wrap the final proof (KoalaBear-in-BN254), so there is no native KoalaBear
// mode.
//
// The [KoalaBearAPI] type provides arithmetic operations for both [Element] and [Ext]:
//   - Base field operations: Add, Sub, Mul, MulConst, etc.
//   - Extension field operations: AddExt, SubExt, MulExt, MulConstExt, etc.
//
// # Usage
//
//	type MyCircuit struct {
//	    X, Y circuit.Var
//	    A, B circuit.Ext
//	}
//
//	func (c *MyCircuit) Define(api frontend.API) error {
//	    f := circuit.MustNewAPI(api)
//
//	    // Base field
//	    sum := f.Add(c.X, c.Y)
//
//	    // Extension field
//	    extProd := f.MulExt(c.A, c.B)
//	    return nil
//	}
//
// # Witness Assignment
//
// For witness assignment, use the static constructors:
//
//	witness := MyCircuit{
//	    X: circuit.NewVarFromKoala(x),
//	    A: circuit.NewExt(a),
//	}
package circuit
