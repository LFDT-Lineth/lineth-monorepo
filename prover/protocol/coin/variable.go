package coin

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

/*
Returns a symbolic representation of a random coin to
use it in a symbolic expression. Only supported for field
coin
*/
func (i Info) AsVariable() *symbolic.Expression {
	if i.Type != FieldExt && i.Type != FieldFromSeed && i.Size > 1 {
		utils.Panic("Only supported for single field coins, but %v has type %v size %v", i.Name, i.Size, i.Type)
	}
	return symbolic.NewVariable(i)
}

func (i Info) String() string {
	/*
		The name is already disambiguated, coin is to avoid conflict
		if we have another instance of `symbolic.Metadata` which has
		the same name but not the same type.
	*/
	return fmt.Sprintf("__COIN__%v", i.Name)
}

/*
IsBase always returns false because coins are always
either field extensions or integer vectors, but not base field elements.*
*/
func (i Info) IsBase() bool {
	/*
	   Coins are always field extensions
	*/
	return false
}
