package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/google/uuid"
)

var _ ifaces.Query = MiMC{}

/*
A MiMC query over a set of 3 columns (block, oldState, newState) enforces
that newState is the result of applying the MiMC compression function to
block and oldState.

We use the MiMC as specified in the following paper.
https://eprint.iacr.org/2016/492.pdf

And the compression function uses the Miyaguchi's construction
https://en.wikipedia.org/wiki/One-way_compression_function#Miyaguchi.E2.80.93Preneel
*/
type MiMC struct {

	// The columns on which the query applies
	Blocks, OldState, NewState ifaces.Column
	// Selector is an optional column that disables the query on rows where the selector is 0
	Selector ifaces.Column
	// The name of the query
	ID   ifaces.QueryID
	uuid uuid.UUID `serde:"omit"`
}

// Name implements the [ifaces.Query] interface
func (m MiMC) Name() ifaces.QueryID {
	return m.ID
}

/*
Constructs a new MiMC query
*/
func NewMiMC(id ifaces.QueryID, block, oldState, newState ifaces.Column, selector ifaces.Column) MiMC {

	/*
		Sanity-check : the querie's ifaces.QueryID cannot be empty or nil
	*/
	if len(id) <= 0 {
		utils.Panic("Given an empty ifaces.QueryID for global constraint query")
	}

	/*
		Sanity-check : All columns must have the same length
	*/
	if block.Size() != oldState.Size() || block.Size() != newState.Size() {
		utils.Panic("block, oldState and newState must have the same length %v %v %v", block.Size(), oldState.Size(), newState.Size())
	}

	if selector != nil && selector.Size() != block.Size() {
		utils.Panic("selector and block must have the same length %v %v", selector.Size(), block.Size())
	}

	return MiMC{
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
func (m MiMC) Check(run ifaces.Runtime) error {

	var (
		blocks                         = m.Blocks.GetColAssignment(run)
		oldStates                      = m.OldState.GetColAssignment(run)
		newStates                      = m.NewState.GetColAssignment(run)
		selector  ifaces.ColAssignment = smartvectors.NewConstant(field.One(), blocks.Len())
	)

	if m.Selector != nil {
		selector = m.Selector.GetColAssignment(run)
	}

	for i := 0; i < newStates.Len(); i++ {

		sel := selector.Get(i)
		if sel.IsZero() {
			continue
		}

		block := blocks.Get(i)
		oldState := oldStates.Get(i)
		newState := newStates.Get(i)

		recomputed := mimc.BlockCompression(oldState, block)
		if recomputed != newState {
			return fmt.Errorf(
				"MiMC compression check failed for row #%v : block %v, oldState %v, newState %v",
				i, block.String(), oldState.String(), newState.String(),
			)
		}
	}

	return nil
}

// Check the mimc relation in a gnark circuit
func (m MiMC) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {

	blocks := m.Blocks.GetColAssignmentGnark(run)
	oldStates := m.OldState.GetColAssignmentGnark(run)
	newStates := m.NewState.GetColAssignmentGnark(run)

	for i := 0; i < len(newStates); i++ {
		block := blocks[i]
		oldState := oldStates[i]
		newState := newStates[i]
		recomputed := mimc.GnarkBlockCompression(api, oldState, block)
		api.AssertIsEqual(newState, recomputed)
	}
}

func (m MiMC) UUID() uuid.UUID {
	return m.uuid
}
