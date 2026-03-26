package symbolic

import (
	"fmt"
	"reflect"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
	"golang.org/x/crypto/blake2b"
)

// Metadata is an interface that must be implemented by type in order to be
// used to instantiate a [Variable] with them.
type Metadata interface {
	/*
		Strings allows addressing a map by variable 2 instances for which
		String() returns the same result are treated as equal.
	*/
	String() string
	IsBase() bool
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

// EvaluateExt implements the [Operator] interface. Yet, this panics if this is called.
func (v Variable) EvaluateExt([]sv.SmartVector) sv.SmartVector {
	panic("we never call it for variables")
}

func (v Variable) EvaluateMixed([]sv.SmartVector) sv.SmartVector {
	panic("we never call it for variables")
}

// GnarkEval implements the [Operator] interface. Yet, this panics if this is called.
func (v Variable) GnarkEval(api frontend.API, inputs []koalagnark.Element) koalagnark.Element {
	panic("we never call it for variables")
}

// GnarkEval implements the [Operator] interface. Yet, this panics if this is called.
func (v Variable) GnarkEvalExt(api frontend.API, inputs []any) koalagnark.Ext {
	panic("we never call it for variables")
}

// NewVariable constructs a new variable object from a parameter implementing
// the [Metadata] interface.
func NewVariable(metadata Metadata) *Expression {
	return &Expression{
		Children: []*Expression{},
		ESHash:   metadataToESH(metadata),
		Operator: Variable{Metadata: metadata},
		IsBase:   metadata.IsBase(),
	}
}

// metadataToESH gets the ESH from a metadata. It is obtained by hashing
// the string representation of the metadata.
func metadataToESH(m Metadata) esHash {
	// since we use bloack2b to hash the string, it is enough to do that on a base field element esh
	sigSeed := []byte(m.String())
	hasher, _ := blake2b.New256(nil)
	hasher.Write(sigSeed)
	h := hasher.Sum(nil)
	r := fext.Element{}
	r.B0.A0.SetBytes(h[0:4])
	r.B0.A1.SetBytes(h[4:8])
	r.B1.A0.SetBytes(h[8:12])
	r.B1.A1.SetBytes(h[12:16])
	return r
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

/*
IsBase is always true for the StringVar testing struct
*/
func (s StringVar) IsBase() bool { return true }

/*
StringVarExt is an implementation of [Metadata] aimed toward testing, but
StringVarExt will be over field extensions.
*/
type StringVarExt string

func NewDummyVarExt(s string) *Expression {
	return NewVariable(StringVarExt(s))
}

func (s StringVarExt) String() string { return string(s) }

/*
IsBase is always false for the StringVarExt testing struct,
as it is considered over field extensions
*/
func (s StringVarExt) IsBase() bool { return false }
