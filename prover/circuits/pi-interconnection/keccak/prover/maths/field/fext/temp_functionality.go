package fext

import "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"

/*
Currently, this function only sets the first coordinate of the field extension
*/
func NewFromString(s string) (res Element) {
	elem := field.NewFromString(s)
	return Element{elem, field.Zero()}
}
