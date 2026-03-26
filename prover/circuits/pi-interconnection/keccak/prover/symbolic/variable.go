package symbolic

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"golang.org/x/crypto/blake2b"
)

// Metadata is an interface that must be implemented by type in order to be
// used to instantiate a [Variable] with them.
type Metadata interface {
	/*
		Strings allows adressing a map by variable 2 instances for which
		String() returns the same result are treated as equal.
	*/
	String() string
}

// Variable implements the [Operator] interface and implements a variable; i.e.
// an abstract value that can be assigned in order to evaluate the expression.
type Variable struct {
	Metadata Metadata
}

// Degree implements the [Operator] interface. Yet, this panics if this is called.
func (Variable) Degree([]int) int {
	panic("we never call it for a variable")
}

// Evaluate implements the [Operator] interface. Yet, this panics if this is called.
func (v Variable) Evaluate([]sv.SmartVector) sv.SmartVector {
	panic("we never call it for variables")
}

// GnarkEval implements the [Operator] interface. Yet, this panics if this is called.
func (v Variable) GnarkEval(api frontend.API, inputs []frontend.Variable) frontend.Variable {
	panic("we never call it for variables")
}

// NewVariable constructs a new variable object from a parameter implementing
// the [Metadata] interface.
func NewVariable(metadata Metadata) *Expression {
	return &Expression{
		Children: []*Expression{},
		ESHash:   metadataToESH(metadata),
		Operator: Variable{Metadata: metadata},
	}
}

// metadataToESH gets the ESH from a metadata. It is obtained by hashing
// the string representation of the metadata.
func metadataToESH(m Metadata) field.Element {
	var esh field.Element
	sigSeed := []byte(m.String())
	hasher, _ := blake2b.New256(nil)
	hasher.Write(sigSeed)
	sigBytes := hasher.Sum(nil)
	esh.SetBytes(sigBytes)
	return esh
}

/*
StringVar is an implementation of [Metadata] aimed toward testing.
*/
type StringVar string

func NewDummyVar(s string) *Expression {
	return NewVariable(StringVar(s))
}

func (s StringVar) String() string { return string(s) }

/*
Validate implements the [Operator] interface.
*/
func (v Variable) Validate(expr *Expression) error {
	// This test that the variable is indeed the operator of the expression it is
	// tested on.
	if !reflect.DeepEqual(v, expr.Operator) {
		utils.Panic("expr.operator %#v != v %#v", v, expr.Operator)
	}

	if len(expr.Children) != 0 {
		return fmt.Errorf("mismatch in the size of the children and coefficients")
	}

	return nil
}
