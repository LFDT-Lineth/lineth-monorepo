package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// columnSizeMaxSupported is the maximum supported size of a column. The value
// comes from the 2-adicity of the koalabear field and having an upper-bound
// is required to permit the construction of the global constraints quotient.
const columnSizeMaxSupported = 1 << 22

// Runtime is the execution context for protocol [ProverAction]s. It holds column
// assignments, cell values, coin values, and an arbitrary state bag. A single
// Runtime serves both prover-side (assign) and verifier-side (read) usage
// through the same [ProverAction] interface.
//
// Pass Runtime by value: all mutable storage lives in map and pointer fields
// (reference types), so mutations made inside an [ProverAction] propagate to the
// caller. The sole exception is [Runtime.AdvanceRound], which must be called
// on a pointer to update [Runtime.currentRound].
type Runtime struct {
	// System is the protocol specification this Runtime executes against.
	System *System
	// currentRound is the round currently being processed.
	currentRound *Round
	// fs is the Fiat-Shamir state. It is updated with column and cell
	// assignments at the end of each round and used to derive coin values for
	// the next round.
	fs *fiatshamir.FiatShamir
	// columns maps each column's [ObjectID] to its concrete vector assignment.
	columns map[ObjectID]*ConcreteVector
	// cells maps each cell's [ObjectID] to its concrete scalar value.
	cells map[ObjectID]field.Gen
	// coins maps each coin's [ObjectID] to its sampled coin value.
	coins map[ObjectID]field.Gen
	// state is a free-form key-value store for stateful actions.
	state map[string]any
	// dynamicSizes maps the index of each dynamic module to its domain size for
	// this Runtime. Populated lazily by [Runtime.AssignColumn] on the first
	// column assignment to each dynamic module.
	dynamicSizes map[int]int
}

// NewRuntime creates a fresh Runtime for sys. currentRound is initialised to
// the first interactive round. Precomputed column assignments from
// [System.PrecomputedRound] are pre-loaded.
//
// Panics if sys has no interactive rounds (len(sys.Rounds) == 0).
func NewRuntime(sys *System) Runtime {
	run := Runtime{
		System:       sys,
		fs:           fiatshamir.NewFiatShamir(),
		columns:      make(map[ObjectID]*ConcreteVector),
		cells:        make(map[ObjectID]field.Gen),
		coins:        make(map[ObjectID]field.Gen),
		state:        make(map[string]any),
		dynamicSizes: make(map[int]int),
	}
	if len(sys.Rounds) == 0 {
		panic("wiop: NewRuntime: system has no interactive rounds")
	}
	run.currentRound = sys.Rounds[0]
	pr := sys.PrecomputedRound
	for i, col := range pr.Columns {
		run.columns[col.Context.ID] = pr.PrecomputedValues[i]
	}
	return run
}

// dynamicModuleSize returns the domain size registered for m in this Runtime.
// Called by [Module.RuntimeSize] for dynamic modules.
func (run Runtime) dynamicModuleSize(m *Module) int {
	size, ok := run.dynamicSizes[m.index]
	if !ok {
		panic(fmt.Sprintf(
			"wiop: dynamic module %q has no size yet in this runtime; assign a column first",
			m.Context.Path(),
		))
	}
	return size
}

// CurrentRound returns the round currently being processed, or nil if the
// system has no interactive rounds.
func (run Runtime) CurrentRound() *Round { return run.currentRound }

// AdvanceRound closes the current round and opens the next one:
//  1. Every oracle or public column assigned in the current round is fed into
//     the Fiat-Shamir state.
//  2. Every cell value assigned in the current round is fed into the
//     Fiat-Shamir state. All cells are always public (see [Cell.Visibility]).
//  3. The runtime advances to the next round.
//  4. A fresh extension-field coin is derived via [fiatshamir.FiatShamir.RandomFext]
//     for each [CoinField] declared in the new round.
//
// Panics if there is no next round, or if any oracle/public column in the
// current round has not been assigned.
func (run *Runtime) AdvanceRound() {
	if run.currentRound == nil {
		panic("wiop: AdvanceRound: system has no interactive rounds")
	}
	next, ok := run.currentRound.Next()
	if !ok {
		panic(fmt.Sprintf(
			"wiop: AdvanceRound: already at the last round (id=%d)",
			run.currentRound.ID,
		))
	}

	// Feed oracle and public column assignments into the Fiat-Shamir state.
	for _, col := range run.currentRound.Columns {
		if col.Visibility < VisibilityOracle {
			continue
		}
		cv := run.GetColumnAssignment(col) // panics if unassigned
		for _, chunk := range cv.Plain {
			run.fs.UpdateSV(chunk)
		}
	}

	// Feed all cell values into the Fiat-Shamir state.
	for _, cell := range run.currentRound.Cells {
		v, ok := run.cells[cell.Context.ID]
		if !ok {
			panic(fmt.Sprintf(
				"wiop: AdvanceRound: cell %q not assigned before advancing round",
				cell.Context.Path(),
			))
		}
		run.fs.UpdateGeneric(v)
	}

	run.currentRound = next

	// Derive a coin for every CoinField declared in the new round.
	for _, coin := range run.currentRound.Coins {
		run.coins[coin.Context.ID] = field.ElemFromExt(run.fs.RandomFext())
	}
}

// AssignColumn stores a concrete vector assignment for col.
//
// Size semantics:
//   - Static module: the data length must not exceed the module's declared size.
//   - Dynamic module: each time we add a column we potentially grow the
//     module's size. There is no upper bound.
func (run Runtime) AssignColumn(col *Column, v *ConcreteVector) {

	if col.round != run.currentRound {
		panic(fmt.Sprintf(
			"wiop: AssignColumn: column %q belongs to round %d but current round is %v",
			col.Context.Path(), col.round.ID, run.currentRound,
		))
	}

	id := col.Context.ID
	if _, exists := run.columns[id]; exists {
		panic(fmt.Sprintf(
			"wiop: AssignColumn: column %q already assigned",
			col.Context.Path(),
		))
	}

	m := col.Module
	dataLen := v.Plain[0].Len()

	if dataLen > columnSizeMaxSupported {
		utils.Panic("wiop: AssignColumn: data length too large for column: %v, size=%v", dataLen, columnSizeMaxSupported)
	}

	if m.IsDynamic() {
		currSize := run.dynamicSizes[m.index]
		run.dynamicSizes[m.index] = max(currSize, dataLen)
	} else if m.IsSized() && dataLen > m.Size() {
		panic(fmt.Sprintf(
			"wiop: AssignColumn: column %q has data length %d which overflows module %q size %d",
			col.Context.Path(), dataLen, m.Context.Path(), m.Size(),
		))
	}

	run.columns[id] = v
}

// GetColumnAssignment returns the concrete assignment of col. Panics if col
// has not been assigned yet.
func (run Runtime) GetColumnAssignment(col *Column) *ConcreteVector {
	v, ok := run.columns[col.Context.ID]
	if !ok {
		panic(fmt.Sprintf(
			"wiop: GetColumnAssignment: column %q is not assigned",
			col.Context.Path(),
		))
	}
	return v
}

// HasColumnAssignment reports whether col has been assigned in this runtime.
func (run Runtime) HasColumnAssignment(col *Column) bool {
	_, ok := run.columns[col.Context.ID]
	return ok
}

// HasCellValue reports whether cell has been assigned in this runtime.
func (run Runtime) HasCellValue(cell *Cell) bool {
	_, ok := run.cells[cell.Context.ID]
	return ok
}

// AssignCell stores a concrete scalar value for cell. Panics if cell does not
// belong to the current round or has already been assigned.
func (run Runtime) AssignCell(cell *Cell, v field.Gen) {
	if cell.round != run.currentRound {
		panic(fmt.Sprintf(
			"wiop: AssignCell: cell %q belongs to round %d but current round is %v",
			cell.Context.Path(), cell.round.ID, run.currentRound,
		))
	}
	id := cell.Context.ID
	if _, exists := run.cells[id]; exists {
		panic(fmt.Sprintf(
			"wiop: AssignCell: cell %q already assigned",
			cell.Context.Path(),
		))
	}
	run.cells[id] = v
}

// GetCellValue returns the concrete scalar value of cell. Panics if cell has
// not been assigned yet.
func (run Runtime) GetCellValue(cell *Cell) field.Gen {
	v, ok := run.cells[cell.Context.ID]
	if !ok {
		panic(fmt.Sprintf(
			"wiop: GetCellValue: cell %q is not assigned",
			cell.Context.Path(),
		))
	}
	return v
}

// HasCellAssignment reports whether cell has been assigned in this runtime.
func (run Runtime) HasCellAssignment(cell *Cell) bool {
	_, ok := run.cells[cell.Context.ID]
	return ok
}

// GetCoinValue returns the value sampled for coin by [Runtime.AdvanceRound].
// Panics if the round containing coin has not been entered yet.
func (run Runtime) GetCoinValue(coin *CoinField) field.Gen {
	v, ok := run.coins[coin.Context.ID]
	if !ok {
		panic(fmt.Sprintf(
			"wiop: GetCoinValue: coin %q has not been sampled yet; call AdvanceRound first",
			coin.Context.Path(),
		))
	}
	return v
}

// GetState returns the value stored under key and whether it was present.
func (run Runtime) GetState(key string) (any, bool) {
	v, ok := run.state[key]
	return v, ok
}

// SetState stores value under key in the runtime's state bag.
func (run Runtime) SetState(key string, value any) {
	run.state[key] = value
}
