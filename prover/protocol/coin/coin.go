package coin

import (
	"fmt"
	"strconv"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/google/uuid"
)

// Wrapper type for naming the coins
type Name string

// Utility function to format names
func Namef(s string, args ...interface{}) Name {
	return Name(fmt.Sprintf(s, args...))
}

// MarshalJSON implements [json.Marshaler] directly returning the name as a
// quoted string.
func (n *Name) MarshalJSON() ([]byte, error) {
	var (
		nString = string(*n)
		nQuoted = strconv.Quote(nString)
	)
	return []byte(nQuoted), nil
}

// UnmarshalJSON implements [json.Unmarshaler] directly assigning the receiver's
// value from the unquoted string value of the bytes.
func (n *Name) UnmarshalJSON(b []byte) error {
	var (
		nQuoted        = string(b)
		nUnquoted, err = strconv.Unquote(nQuoted)
	)

	if err != nil {
		return fmt.Errorf("could not unmarshal Name from unquoted string: %v : %w", nQuoted, err)
	}

	*n = Name(nUnquoted)
	return nil
}

// Metadata around the random coin. The struct is made non-constructible to
// ensure that it is built from the [NewInfo] constructor. The reason is that
// the structure contains an internal UUID used for the serialization process
// and we want to be sure that all instances have their own UUID.
type Info struct {
	info
}

type info struct {
	Type Type `json:"type"`
	// Set if applicable (for instance, IntegerVec)
	Size int `json:"size"`
	// Upper-bound (if applicable, for instance for integers)
	UpperBound int `json:"upperBound"`
	// Name of the coin
	Name Name `json:"name"`
	// Round at which the coin was declared
	Round int `json:"round"`
	// uuid is a pointer to the compiler IOP. This is used as part of the
	// serialization process
	uuid uuid.UUID
}

// Type of random coin
type Type int

const (
	Field Type = iota
	IntegerVec
	FieldFromSeed
)

// MarshalJSON implements [json.Marshaler] directly returning the Itoa of the
// integer.
func (t Type) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(t))), nil
}

// UnmarshalJSON implements [json.Unmarshaler] and directly reuses ParseInt and
// performing validation : only 0 and 1 are acceptable values.
func (t *Type) UnmarshalJSON(b []byte) error {
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse Type as integer: %w, got `%v`", err, string(b))
	}

	if n < 0 || Type(n) > IntegerVec {
		return fmt.Errorf("could not parse the integer `%v` as Type, must be in range [0, 1]", n)
	}

	*t = Type(n)
	return nil
}

/*
Sample a random coin, according to its `spec`
*/
func (info *Info) Sample(fs *fiatshamir.State, seed field.Element) interface{} {
	switch info.Type {
	case Field:
		return fs.RandomField()
	case IntegerVec:
		return fs.RandomManyIntegers(info.Size, info.UpperBound)
	case FieldFromSeed:
		return fs.RandomFieldFromSeed(seed, string(info.Name))
	}
	panic("Unreachable")
}

// SampleGnark samples a random coin in a gnark circuit. The seed can optionally be
// passed by the caller is used for [FieldFromSeed] coins. The function returns
func (info *Info) SampleGnark(fs *fiatshamir.GnarkFiatShamir, seed frontend.Variable) interface{} {
	switch info.Type {
	case Field:
		return fs.RandomField()
	case IntegerVec:
		return fs.RandomManyIntegers(info.Size, info.UpperBound)
	case FieldFromSeed:
		return fs.RandomFieldFromSeed(seed, string(info.Name))
	}
	panic("Unreachable")
}

/*
Constructor for Info. For IntegerVec, size[0] contains the
number of integers and size[1] contains the upperBound.
*/
func NewInfo(name Name, type_ Type, round int, size ...int) Info {

	infos := Info{info{Name: name, Type: type_, Round: round, uuid: uuid.New()}}

	switch type_ {
	case IntegerVec:
		if len(size) != 2 || size[0] < 1 || size[1] < 1 {
			utils.Panic("caller requested an `IntegerVec` and was expected to provide [nbIntegers, upperBound] and additional parameters but provided `%v`", size)
		}
		infos.Size = size[0]
		infos.UpperBound = size[1]
	case Field:
		if len(size) > 0 {
			utils.Panic("size for Field")
		}
	case FieldFromSeed:
		if len(size) > 0 {
			utils.Panic("size for Field")
		}
	default:
		panic("unreachable")
	}

	return infos
}

func (info Info) UUID() uuid.UUID {
	return info.uuid
}
