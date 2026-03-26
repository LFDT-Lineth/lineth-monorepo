package execution_data_collector

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// PoseidonHasher is used to Poseidon-hash the data in LogMessages. Using a zero initial stata,
// the data in L2L1 logs must be hashed as follows: msg1Hash, msg2Hash, and so on.
// The final value of the chained hash can be retrieved as hash[ctMax[any index]]
type PoseidonHasher struct {
	InputData     [common.NbLimbU128]ifaces.Column
	InputIsActive ifaces.Column
	// the Hash value after each step, as explained in the description of PoseidonHasher
	Hash [common.NbLimbU128]ifaces.Column
	// L2L1 logs: Inter is a shifted version of hashSecond, necessary due to how the MiMC constraints operate
	Inter [common.NbLimbU128]ifaces.Column
	// the relevant value of the hash (the last value when isActive ends)
	HashFinal [common.NbLimbU128]ifaces.Column
}

// NewPoseidonHasher returns a new PoseidonHasher with initialized columns that are not constrained.
func NewPoseidonHasher(comp *wizard.CompiledIOP, InputData [common.NbLimbU128]ifaces.Column, InputIsActive ifaces.Column, name string) PoseidonHasher {
	var res PoseidonHasher

	size := InputData[0].Size()
	res.InputData = InputData
	res.InputIsActive = InputIsActive
	for i := range res.Hash {
		res.Hash[i] = util.CreateCol(name, fmt.Sprintf("HASH_FIRST_%d", i), size, comp)
		res.Inter[i] = util.CreateCol(name, fmt.Sprintf("INTER_%d", i), size, comp)
		res.HashFinal[i] = util.CreateCol(name, fmt.Sprintf("HASH_FINAL_%d", i), size, comp)
	}

	return res
}

// DefinePoseidonHasher specifies the constraints of the PoseidonHasher with respect to the ExtractedData fetched from the arithmetization
func DefinePoseidonHasher(comp *wizard.CompiledIOP, hasher PoseidonHasher, name string) {

	// Needed for the limitless prover to understand that the columns are
	// not just empty with just padding and suboptimal representation.
	for i := range hasher.Hash {
		pragmas.MarkRightPadded(hasher.Hash[i])
		pragmas.MarkRightPadded(hasher.HashFinal[i])
		pragmas.MarkRightPadded(hasher.Inter[i])
	}

	// Inter[0:8] on the first row is the initial state, which is zero
	// we hash the first 8 limbs and place the data into the Inter[8:16] columns
	comp.InsertPoseidon2(0, ifaces.QueryIDf("%s_%s", name, "POSEIDON2_QUERY"),
		hasher.InputData,
		hasher.Inter,
		hasher.Hash,
		hasher.InputIsActive,
	)

	for i := range hasher.Hash {
		// intermediary state integrity
		comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s_%d", name, "CONSISTENCY_INTER_AND_HASH_SECOND", i), // LAST is either hashSecond or hashThird
			sym.Mul(
				sym.Sub(
					column.Shift(hasher.Hash[i], -1),
					hasher.Inter[i],
				),
				hasher.InputIsActive,
			),
		)

		// inter, the old state column, is initially zero
		comp.InsertLocal(
			0, ifaces.QueryIDf("%s_INTER_LOCAL_%d", name, i),
			ifaces.ColumnAsVariable(hasher.Inter[i]),
		)

		// constrain HashFinal
		commonconstraints.MustBeConstant(comp, hasher.HashFinal[i])
		util.CheckLastELemConsistency(comp, hasher.InputIsActive, hasher.Hash[i], hasher.HashFinal[i], name)
	}
}

// AssignPoseidonHasher assigns the data in the PoseidonHasher using the ExtractedData fetched from the arithmetization
func AssignPoseidonHasher(run *wizard.ProverRuntime, hasher PoseidonHasher, InputData [common.NbLimbU128]ifaces.Column, InputIsActive ifaces.Column) {
	size := InputData[0].Size()
	var hash, inter [8]*common.VectorBuilder

	for index := range hasher.Hash {
		hash[index] = common.NewVectorBuilder(hasher.Hash[index])
		inter[index] = common.NewVectorBuilder(hasher.Inter[index])
	}

	var (
		fetchedData [common.NbLimbU128]field.Element
		hashFinal   [common.NbLimbU128]field.Element
	)

	zeroedState := make([]field.Element, common.NbLimbU128)
	for i := range zeroedState {
		zeroedState[i].SetZero()
	}
	state := vortex.Hash(zeroedState) // the initial state is zero

	for i := 0; i < size; i++ {

		isActive := InputIsActive.GetColAssignmentAt(run, i)
		if isActive.IsZero() {
			break
		}

		for j := range fetchedData {
			fetchedData[j] = InputData[j].GetColAssignmentAt(run, i)
		}

		// Inter is either the initial state so (zero) for row i=0 or the last
		// state from row i-1
		for j := range hasher.Inter {
			inter[j].PushField(state[j])
		}

		// first, hash the first 8 bytes part of the fetched log message
		state = vortex.CompressPoseidon2(vortex.Hash(state), vortex.Hash(fetchedData[0:8]))
		for j := range hash {
			hash[j].PushField(state[j])
		}

		// continuously update HashFinal. Its value will be the last value of
		// hash where isActive ends
		for j := range hashFinal {
			hashFinal[j].Set(&state[j])
		}

	}

	// assign the hasher columns
	for index := range hasher.Hash {
		inter[index].PadAndAssign(run, field.Zero())
		hash[index].PadAndAssign(run, field.Zero())
		run.AssignColumn(hasher.HashFinal[index].GetColID(), smartvectors.NewConstant(hashFinal[index], size))
	}
}
