// Package koalagnark provides circuit (gnark) representations of KoalaBear field elements.
//
// This package contains two main types:
//   - [Var]: A circuit variable over the KoalaBear base field
//   - [Ext]: A circuit variable over the degree-4 extension field
//
// These types abstract over native and emulated arithmetic, allowing the same
// circuit code to work in both native KoalaBear circuits and emulated circuits
// (e.g., BLS12-377).
//
// The [API] type provides arithmetic operations for both [Element] and [Ext]:
//   - Base field operations: Add, Sub, Mul, MulConst, etc.
//   - Extension field operations: AddExt, SubExt, MulExt, MulConstExt, etc.
//
// # Usage
//
//	type MyCircuit struct {
//	    X, Y koalagnark.Var
//	    A, B koalagnark.Ext
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
//	    X: koalagnark.NewVarFromKoala(x),
//	    A: koalagnark.NewExt(a),
//	}
package koalagnark
