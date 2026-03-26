package keccakfkoalabear

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	kcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

type KeccakfInputs struct {
	Blocks       [][kcommon.NumSlices]ifaces.Column // the blocks of the message to be absorbed. first blocks of messages are located in positions 0 mod 24 and are represented in base clean 12, other blocks of message are located in positions 23 mod 24 and are represented in base clean 11. otherwise the blocks are zero.
	IsBlock      ifaces.Column                      // indicates whether the row corresponds to a block
	IsFirstBlock ifaces.Column
	IsBlockBaseB ifaces.Column
	IsActive     ifaces.Column // active part of the blocks (technicaly it is the active part of the keccakf module).
	KeccakfSize  int           // number of keccakf permutations to be proved.
}

// Wizard module responsible for proving a sequence of keccakf permutation
type Module struct {
	Inputs KeccakfInputs
	// initial state before applying the keccakf rounds.
	InitialState kcommon.State
	// Theta module, responsible for updating the state in the Theta step of keccakf
	Theta *theta
	// rho pi module, responsible for updating the state in the rho and pi steps of keccakf
	RhoPi *rhoPi
	// chi module, responsible for updating the state in the chi step of keccakf
	ChiIota *chiIota
	// module to prepare the state to go  back to theta or output the hash result.
	BackToThetaOrOutput *BackToThetaOrOutput
}

// NewModule creates a new keccakf module, declares the columns and constraints and returns its pointer
func NewModule(comp *wizard.CompiledIOP, in KeccakfInputs) *Module {

	var (
		initialState kcommon.State
	)

	// create the initial state, before applying a kevccakf round.
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < kcommon.NumSlices; z++ {
				initialState[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("INITIAL_STATE_%v_%v_%v", x, y, z), in.KeccakfSize, true)
			}
		}
	}

	// assign the first blocks of the message to the initialState
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			m := 5*y + x
			if m < kcommon.NumLanesInBlock {
				for z := 0; z < kcommon.NumSlices; z++ {
					// if it is the first block of the message, assign it to the state
					comp.InsertGlobal(0, ifaces.QueryIDf("STATE_IS_SET_TO_FIRST_BLOCK_%v_%v_%v", x, y, z),
						sym.Mul(in.IsFirstBlock,
							sym.Sub(initialState[x][y][z], in.Blocks[m][z]),
						),
					)
				}
			} else {
				for z := 0; z < kcommon.NumSlices; z++ {
					//  the remaining columns of the state are set to zero
					comp.InsertGlobal(0, ifaces.QueryIDf("STATE_IS_SET_TO_ZERO_%v_%v_%v", x, y, z),
						sym.Mul(in.IsFirstBlock, initialState[x][y][z]),
					)
				}
			}
		}
	}

	// create the theta module with the initial state as input
	theta := newTheta(comp, in.KeccakfSize, initialState)
	// create the rho module with the state after theta
	rhoPi := newRho(comp, theta.StateNext)
	// create the chi iota module with the state after rhoPi
	chiIota := newChi(comp, chiInputs{
		StateCurr:    *rhoPi.StateNext,
		Blocks:       block(in.Blocks),
		IsBlockOther: in.IsBlockBaseB,
		KeccakfSize:  in.KeccakfSize,
	})

	// prepare the state to back to theta or output the hash result.
	thetaOrOutput := newBackToThetaOrOutput(comp, chiIota.StateNext, in.IsActive, in.IsFirstBlock)

	// get the the final state
	finalState := thetaOrOutput.StateNext

	// initialState is set to the previous final state.
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < kcommon.NumSlices; z++ {
				comp.InsertGlobal(0, ifaces.QueryIDf("STATE_IS_CARRIED_OVER_%v_%v_%v", x, y, z),
					sym.Mul(in.IsActive,
						sym.Sub(1, in.IsFirstBlock),
						sym.Sub(initialState[x][y][z], column.Shift(finalState[x][y][z], -1))),
				)
			}
		}
	}

	return &Module{
		Inputs:              in,
		InitialState:        initialState,
		Theta:               theta,
		RhoPi:               rhoPi,
		ChiIota:             chiIota,
		BackToThetaOrOutput: thetaOrOutput,
	}
}

// Assign the values to the columns of the keccakf module.
func (m *Module) Assign(run *wizard.ProverRuntime, traces keccak.PermTraces) {

	m.assignState(run, traces)                   // assign the initial state
	m.Theta.assignTheta(run, m.InitialState)     // assign the theta module with the state
	m.RhoPi.assignRho(run, m.Theta.StateNext)    // assign the rho pi module with the state after theta
	m.ChiIota.assignChi(run, *m.RhoPi.StateNext) // assign the chi iota module with the state after rho pi
	m.BackToThetaOrOutput.Run(run)               // assign the
}

// Returns the number of rows required to prove `numKeccakf` calls to the
// permutation function. The result is padded to the next power of 2 in order to
// satisfy the requirements of the Wizard to have only powers of 2.
func NumRows(numKeccakf int) int {
	return utils.NextPowerOfTwo(numKeccakf * kcommon.NumRounds)
}

// Assigns the state of the module using the keccak traces.
func (mod *Module) assignState(
	run *wizard.ProverRuntime,
	traces keccak.PermTraces,
) {

	var (
		state      = [5][5][kcommon.NumSlices]*common.VectorBuilder{}
		numKeccakF = len(traces.KeccakFInps)
	)

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < kcommon.NumSlices; z++ {
				state[x][y][z] = common.NewVectorBuilder(mod.InitialState[x][y][z])
			}
		}
	}

	for nperm := 0; nperm < numKeccakF; nperm++ {
		// Fetch the current's permutation actual input state. Observe
		// that keccak.State is not a pointer to the data, so this is
		// actually a deep-copy operation. And the later mutations of
		// currInp have no side-effects on the traces.
		currInp := traces.KeccakFInps[nperm]
		currOut := traces.KeccakFOuts[nperm]

		for r := 0; r < keccak.NumRound; r++ {
			// Assign the state to the input columns
			for x := 0; x < 5; x++ {
				for y := 0; y < 5; y++ {

					lanebytes := [kcommon.NumSlices]uint8{}
					for j := 0; j < kcommon.NumSlices; j++ {
						lanebytes[j] = uint8((currInp[x][y] >> (kcommon.NumSlices * j)) & 0xff)
					}
					// convert each byte to clean base common.common.BaseTheta
					for j := 0; j < kcommon.NumSlices; j++ {
						state[x][y][j].PushField(kcommon.U64ToBaseX(uint64(lanebytes[j]), &kcommon.BaseThetaFr))
					}
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
	// assign the values to the state columns
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < kcommon.NumSlices; z++ {
				state[x][y][z].PadAndAssign(run)
			}
		}
	}

}
