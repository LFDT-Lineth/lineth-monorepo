package query

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/google/uuid"
)

var _ ifaces.Query = Poseidon2{}

/*
A Poseidon2 query over a set of 3*8 columns (block, oldState, newState) enforces
that newState is the result of applying the Poseidon2 compression function to
block and oldState.
*/
type Poseidon2 struct {

	// The columns on which the query applies
	Blocks, OldState, NewState [8]ifaces.Column
	// Selector is an optional column that disables the query on rows where the selector is 0
	Selector ifaces.Column
	// The name of the query
	ID   ifaces.QueryID
	uuid uuid.UUID `serde:"omit"`
}

// Name implements the [ifaces.Query] interface
func (p Poseidon2) Name() ifaces.QueryID {
	return p.ID
}

/*
Constructs a new Poseidon2 query
*/
func NewPoseidon2(id ifaces.QueryID, block, oldState, newState [8]ifaces.Column, selector ifaces.Column) Poseidon2 {

	/*
		Sanity-check : the querie's ifaces.QueryID cannot be empty or nil
	*/
	if len(id) <= 0 {
		utils.Panic("Given an empty ifaces.QueryID for global constraint query")
	}

	/*
		Sanity-check : All columns must have the same length
	*/
	for i := range block {
		if block[i] == nil {
			utils.Panic("block is nil: %v", i)
		}

		if oldState[i] == nil {
			utils.Panic("oldState is nil: %v", i)
		}

		if newState[i] == nil {
			utils.Panic("newState is nil: %v", i)
		}

		if block[i].Size() != oldState[i].Size() || block[i].Size() != newState[i].Size() {
			utils.Panic("block, oldState and newState must have the same length %v %v %v", block[i].Size(), oldState[i].Size(), newState[i].Size())
		}
		if selector != nil && selector.Size() != block[i].Size() {
			utils.Panic("selector and block must have the same length %v %v", selector.Size(), block[i].Size())
		}
	}

	return Poseidon2{
		OldState: oldState,
		NewState: newState,
		Blocks:   block,
		Selector: selector,
		ID:       id,
		uuid:     uuid.New(),
	}
}

/*
The verifier checks that the permutation was applied correctly
*/
func (p Poseidon2) Check(run ifaces.Runtime) error {

	var blocks, oldStates, newStates [8]smartvectors.SmartVector
	for i := 0; i < 8; i++ {
		blocks[i] = p.Blocks[i].GetColAssignment(run)
		oldStates[i] = p.OldState[i].GetColAssignment(run)
		newStates[i] = p.NewState[i].GetColAssignment(run)
	}
	var (
		selector ifaces.ColAssignment = smartvectors.NewConstant(field.One(), blocks[0].Len())
	)

	if p.Selector != nil {
		selector = p.Selector.GetColAssignment(run)
	}

	for i := 0; i < newStates[0].Len(); i++ {

		sel := selector.Get(i)
		if sel.IsZero() {
			continue
		}

		var block field.Octuplet
		var oldState field.Octuplet
		var newState field.Octuplet
		for j := 0; j < 8; j++ {
			block[j] = blocks[j].Get(i)
			oldState[j] = oldStates[j].Get(i)
			newState[j] = newStates[j].Get(i)
		}

		recomputed := vortex.CompressPoseidon2(oldState, block)
		if recomputed != newState {
			return fmt.Errorf(
				"Poseidon2 compression [%v] check failed for row #%v : block=%v, oldState=%v, newState=%v, recomputed=%v\n",
				p.ID,
				i,
				types.KoalaOctuplet(block).Hex(),
				types.KoalaOctuplet(oldState).Hex(),
				types.KoalaOctuplet(newState).Hex(),
				types.KoalaOctuplet(recomputed).Hex(),
			)
		}
	}

	return nil
}

// Check the mimc relation in a gnark circuit
func (p Poseidon2) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {

	var blocks, oldStates, newStates [8][]koalagnark.Element
	for i := 0; i < 8; i++ {
		blocks[i] = p.Blocks[i].GetColAssignmentGnark(run)
		oldStates[i] = p.OldState[i].GetColAssignmentGnark(run)
		newStates[i] = p.NewState[i].GetColAssignmentGnark(run)
	}

	for i := 0; i < len(newStates); i++ {
		var block [8]koalagnark.Element
		var oldState [8]koalagnark.Element
		var newState [8]koalagnark.Element
		for j := 0; j < 8; j++ {
			block[j] = blocks[j][i]
			oldState[j] = oldStates[j][i]
			newState[j] = newStates[j][i]
		}
		recomputed := poseidon2_koalabear.GnarkBlockCompressionMekle(api, oldState, block)
		api.AssertIsEqual(newState, recomputed)
	}
}

func (p Poseidon2) UUID() uuid.UUID {
	return p.uuid
}
