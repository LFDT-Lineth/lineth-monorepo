package gnarkutil

import (
	"reflect"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/schema"
)

// CountVariables count the variables of a circuit without compiling it. It returns
// the number of public, secret and internal variables.
func CountVariables(circ any) (nbPublic, nbSecret int) {

	// tVar holds a reference to the reflect.Type of [frontend.Variable]
	var (
		tVar = reflect.ValueOf(struct{ A frontend.Variable }{}).FieldByName("A").Type()
	)

	s, err := schema.Walk(circ, tVar, nil)
	if err != nil {
		panic(err)
	}

	return s.Public, s.Secret
}
