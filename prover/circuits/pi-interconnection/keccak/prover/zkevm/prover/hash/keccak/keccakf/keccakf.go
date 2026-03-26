// The keccakf package implements to keccakf module. It provides Module as a
// main abstraction that can be used as a submodule of a larger Wizard in
// order to prove iterations of the Keccakf sponge function.
package keccakf

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

const (
	// We use BaseA representation of bit slices for the theta phase and BaseB
	// representation for the chi-iota phase.
	BaseA = 12
	BaseB = 11
	// nBitsKeccakLane denotes the number of bits within a Keccakf lane. And
	// numRounds (=24) indicates the number of rounds required to make up a full
	// run of the keccakf permutation function.
	nBitsKeccakLane = 64
	numRounds       = keccak.NumRound // number of rounds in keccakf
	// In order to convert between Base1 (theta) and Base2 (chi-iota), we split
	// the 64 bits representative slices into slices representing 4 bits and we
	// need 16 of them to represent an entire u64. The reason for this choice of
	// 4 and 16 is that it allows us to limit the size of the corresponding
	// lookup tables.
	numSlice      = 16
	numChunkBaseX = nBitsKeccakLane / numSlice
	// The integer version of baseX ^ 4. Usefull for limb decomposition/recompo-
	// sition.
	BaseAPow4 = BaseA * BaseA * BaseA * BaseA
	BaseBPow4 = BaseB * BaseB * BaseB * BaseB
	// Number of 64bits lanes in a keccak block
	numLanesInBlock = 17
)

var (
	// The field version of BaseA / BaseB
	BaseAFr = field.NewElement(BaseA)
	BaseBFr = field.NewElement(BaseB)
	// The field version
	BaseAPow4Fr = field.NewElement(BaseAPow4)
	BaseBPow4Fr = field.NewElement(BaseBPow4)
)

// InputWitnessProvider is a returning a succession of keccak.State when it will
// be time to prove. One restriction is that it should never return more
// permutations than what is actually specified when declaring the module.
type InputWitnessProvider = func() keccak.PermTraces

// Wizard module responsible for proving a sequence of keccakf permutation
type Module struct {
	// Maximal number of Keccakf permutation that the module can handle
	MaxNumKeccakf int

	// The State of the keccakf before starting a new round.
	// Note : unlike the original keccakf where the initial State is zero,
	// the initial State here is the first block of the message.
	State [5][5]ifaces.Column

	// Columns representing the messages blocks hashed with keccak. More
	// Given our implementation it is more efficient to do the
	// xoring in base B (so at the end of the function) and the "initial" XOR
	// cannot be done at this moment. Fortunately, the initial XOR operation is
	// straightforward to handle as an edge-case since it is done against a
	// zero-state. Thus,  the entries of position 23 mod 24 are in Base B
	// and they are zero of the corresponding round is the last round of the
	// last call to the sponge function for a given hash (i.e. there are no more
	// blocks to XORIN and what remains is only to recover the result of the
	// hash). The first block are in Base A located at positions 0 mod 24.
	// At any other position block is zero.
	//  The Keccakf module trusts these columns to be well-formed.
	Blocks [numLanesInBlock]ifaces.Column

	// It is 1 over the effective part of the module
	// (indicating the rows of the module occupied by the witness).
	IsActive ifaces.Column

	// Submodules used to declare the successive steps of the keccak round
	// permutation function.
	IO        InputOutput
	Theta     theta
	Rho       rho
	PiChiIota piChiIota
	// Collection of the lookup tables
	Lookups lookUpTables
}

// Calls the Keccak module
func NewModule(
	comp *wizard.CompiledIOP,
	round, maxNumKeccakf int,
) (mod Module) {

	mod.MaxNumKeccakf = maxNumKeccakf
	// declare the columns
	mod.declareColumns(comp, round, maxNumKeccakf)

	// Initializes the lookup columns
	mod.Lookups = newLookUpTables(comp, maxNumKeccakf)

	// Then initializes the submodules : declare the columns and all the
	// constraints per submodule.
	mod.IO.newInput(comp, maxNumKeccakf, mod)
	mod.Theta = newTheta(comp, round, maxNumKeccakf, mod.State, mod.Lookups)
	mod.Rho = newRho(comp, round, maxNumKeccakf, mod.Theta.AThetaSlicedBaseB)
	mod.PiChiIota = newPiChiIota(comp, round, maxNumKeccakf, mod)
	mod.IO.newOutput(comp, maxNumKeccakf, mod)

	return mod
}

// Registers the prover steps per submodule.
func (mod *Module) Assign(
	run *wizard.ProverRuntime,
	traces keccak.PermTraces,
) {

	// Number of permutation used for the current instance
	numKeccakf := len(traces.KeccakFInps)

	// If the number of keccakf constraints is larger than what the module
	// is sized for, then, we cannot prove everything.
	if numKeccakf > mod.MaxNumKeccakf {
		exit.OnLimitOverflow(
			mod.MaxNumKeccakf,
			numKeccakf,
			fmt.Errorf("too many keccakf %v > %v", numKeccakf, mod.MaxNumKeccakf),
		)
	}

	lu := mod.Lookups
	lu.DontUsePrevAIota.Assign(run)
	mod.assignStateAndBlocks(run, traces, numKeccakf)
	mod.IO.assignBlockFlags(run, traces)
	mod.Theta.assign(run, mod.State, lu, numKeccakf)
	mod.Rho.assign(run, mod.Theta.AThetaSlicedBaseB, numKeccakf)
	mod.PiChiIota.assign(run, numKeccakf, lu, mod.Rho.ARho,
		mod.Blocks, mod.IO.IsBlockBaseB)
	mod.IO.assignHashOutPut(run, mod.IsActive)

}

// Assigns the state of the module using the keccak traces.
func (mod *Module) assignStateAndBlocks(
	run *wizard.ProverRuntime,
	traces keccak.PermTraces,
	numKeccakF int,
) {
	colSize := mod.State[0][0].Size()
	unpaddedSize := numKeccakF * keccak.NumRound

	run.AssignColumn(
		mod.IsActive.GetColID(),
		smartvectors.RightZeroPadded(
			vector.Repeat(field.One(), unpaddedSize),
			colSize,
		),
	)

	// Assign the state from the traces
	inputsVal := [5][5][]field.Element{}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			inputsVal[x][y] = make(
				[]field.Element,
				unpaddedSize,
			)
		}
	}

	// Assign the block in BaseB.  The Xoring with the state
	// is done at the end of the previous call of the sponge function (at its
	// last operation during the iota phase).
	blocksVal := [numLanesInBlock][]field.Element{}
	for m := range blocksVal {
		blocksVal[m] = make([]field.Element, unpaddedSize)
	}

	parallel.Execute(numKeccakF, func(start, stop int) {

		for nperm := start; nperm < stop; nperm++ {
			// Fetch the current's permutation actual input state. Observe
			// that keccak.State is not a pointer to the data, so this is
			// actually a deep-copy operation. And the later mutations of
			// currInp have no side-effects on the traces.
			currInp := traces.KeccakFInps[nperm]
			currOut := traces.KeccakFOuts[nperm]
			currBlock := traces.Blocks[nperm]

			for r := 0; r < keccak.NumRound; r++ {
				// Current row that we are assigning
				currRow := nperm*keccak.NumRound + r

				// Assign the state to the input columns
				for x := 0; x < 5; x++ {
					for y := 0; y < 5; y++ {
						inputsVal[x][y][currRow] = U64ToBaseX(currInp[x][y], &BaseAFr)
					}
				}

				// Retro-actively assign the block in BaseB if we are not on
				// the first row. The condition over nperm is to ensure that we
				// do not underflow although in practice isNewHash[0] will
				// always be true because this is the first perm of the first
				// hash by definition.
				if r == 0 && nperm > 0 && !traces.IsNewHash[nperm] {
					for m := 0; m < numLanesInBlock; m++ {
						blocksVal[m][currRow-1] = U64ToBaseX(currBlock[m], &BaseBFr)
					}
				}
				//assign the firstBlock in BaseA
				if r == 0 && traces.IsNewHash[nperm] {
					for m := 0; m < numLanesInBlock; m++ {
						blocksVal[m][currRow] = U64ToBaseX(currBlock[m], &BaseAFr)
					}
				}

				// Applies one round to mutate currInp into the input
				// corresponding to the next round. That way, next iteration
				// directly accesses the input of the next round.
				currInp.ApplyKeccakfRound(r)

				// At the last round, if everything goes well, we should have
				// the value of keccakfOups
				if r == keccak.NumRound-1 && currInp != currOut {
					utils.Panic(
						"assigned values are inconsistent with the traces\n"+
							"\t%+x\n\t%+v\n",
						currOut, currInp,
					)
				}
			}
		}
	})

	for m := 0; m < numLanesInBlock; m++ {
		run.AssignColumn(
			mod.Blocks[m].GetColID(),
			smartvectors.RightZeroPadded(blocksVal[m], colSize),
		)
	}

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			run.AssignColumn(
				mod.State[x][y].GetColID(),
				smartvectors.RightZeroPadded(inputsVal[x][y], colSize),
			)
		}
	}

}

// declare the columns of the state and the message.
func (mod *Module) declareColumns(comp *wizard.CompiledIOP, round, maxNumKeccakF int) {
	size := numRows(maxNumKeccakF)

	// Initialize the column isActive
	mod.IsActive = comp.InsertCommit(round, deriveName("IS_ACTIVE"), size)

	// Initializes the state columns
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			mod.State[x][y] = comp.InsertCommit(
				round,
				deriveName("A_INPUT", x, y),
				size,
			)
		}
	}

	// Initializes the columns of the message
	for m := 0; m < numLanesInBlock; m++ {
		mod.Blocks[m] = comp.InsertCommit(
			round,
			deriveName("BLOCK_BASE_2", m),
			size,
		)
	}
}
