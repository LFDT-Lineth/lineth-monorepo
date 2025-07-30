package execution_data_collector

import (
	"math"

	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

type MIMCHasher struct {
	// a typical IsActive binary column, provided as an input to the module
	IsActive ifaces.Column
	// the data to be hashed, this column is provided as an input to the module
	InputData      ifaces.Column
	InputIsActive  ifaces.Column
	Data           ifaces.Column
	IsData         ifaces.Column //isActive * canBeData
	IsDataFirstRow *dedicated.HeartBeatColumn
	IsDataOddRows  *dedicated.HeartBeatColumn
	// this column stores the MiMC hashes
	Hash ifaces.Column
	// a constant column that stores the last relevant value of the hash
	HashFinal ifaces.Column
	// State is an intermediary column used to enforce the MiMC constraints
	State ifaces.Column
}

func NewMIMCHasher(comp *wizard.CompiledIOP, inputData, inputIsActive ifaces.Column, name string) *MIMCHasher {
	size := 2 * inputData.Size()
	res := &MIMCHasher{
		InputData:     inputData,
		InputIsActive: inputIsActive,
		Data:          util.CreateCol(name, "DATA", size, comp),
		IsActive:      util.CreateCol(name, "ACTIVE", size, comp),
		Hash:          util.CreateCol(name, "HASH", size, comp),
		HashFinal:     util.CreateCol(name, "HASH_FINAL", size, comp),
		State:         util.CreateCol(name, "STATE", size, comp),
		IsData:        util.CreateCol(name, "IS_DATA", size, comp),
	}
	// Passing a very large size will ensure that the heartbeat produces the
	// same result even if we change the length of the column.
	res.IsDataFirstRow = dedicated.CreateHeartBeat(comp, 0, math.MaxInt, 0, res.IsActive)
	res.IsDataOddRows = dedicated.CreateHeartBeat(comp, 0, 2, 1, res.IsActive)
	return res
}

func DefineHashFilterConstraints(comp *wizard.CompiledIOP, hasher *MIMCHasher, name string) {

	// we require that isActive is binary in DefineIndicatorsMustBeBinary
	// require that the isActive filter only contains 1s followed by 0s
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_IS_ACTIVE_CONSTRAINT_NO_0_TO_1", name),
		sym.Sub(
			hasher.IsActive,
			sym.Mul(
				column.Shift(hasher.IsActive, -1),
				hasher.IsActive,
			),
		),
	)
	util.MustBeBinary(comp, hasher.IsActive)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_IS_DATA", name),
		sym.Sub(
			hasher.IsData,
			hasher.IsDataFirstRow.Natural,
			hasher.IsDataOddRows.Natural,
		),
	)
	util.MustBeBinary(comp, hasher.IsData)
}

// DefineHasher defines the constraints of the MIMCHasher.
// Its isActive and data columns are assumed to be already constrained in another module, no need to constrain them again.
func (hasher *MIMCHasher) DefineHasher(comp *wizard.CompiledIOP, name string) {

	// MiMC constraints
	comp.InsertMiMC(0, ifaces.QueryIDf("%s_%s", name, "MIMC_CONSTRAINT"), hasher.Data, hasher.State, hasher.Hash, nil)

	// intermediary state integrity
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "CONSISTENCY_STATE_AND_HASH_LAST"), // LAST is either hashSecond
		sym.Add(
			sym.Mul(
				hasher.IsData,
				sym.Sub(hasher.State,
					column.Shift(hasher.Hash, -1),
				),
			),
			sym.Mul(
				sym.Sub(1, hasher.IsData),
				sym.Sub(hasher.State,
					0,
				),
			),
		),
	)

	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "CONSISTENCY_STATE_AND_HASH_LAST_2"), // LAST is either hashSecond
		sym.Mul(
			hasher.IsActive,
			sym.Sub(1, hasher.IsData),
			sym.Sub(hasher.Data,
				column.Shift(hasher.Hash, -1),
			),
		),
	)

	// state, the current state column, is initially zero
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "INTER_LOCAL"), ifaces.ColumnAsVariable(hasher.State))

	// constrain HashFinal
	commonconstraints.MustBeConstant(comp, hasher.HashFinal)
	util.CheckLastELemConsistency(comp, hasher.IsActive, hasher.Hash, hasher.HashFinal, name)

	// constraint isActive
	DefineHashFilterConstraints(comp, hasher, name)

	comp.InsertProjection(
		ifaces.QueryIDf("%s_%s", name, "PROJECTION_DATA"),
		query.ProjectionInput{
			ColumnA: []ifaces.Column{hasher.Data},
			ColumnB: []ifaces.Column{hasher.InputData},
			FilterA: hasher.IsData,
			FilterB: hasher.InputIsActive,
		},
	)

}

// AssignHasher assigns the data in the MIMCHasher. The data and isActive columns are fetched from another module.
func (hasher *MIMCHasher) AssignHasher(run *wizard.ProverRuntime) {

	var (
		inputSize = hasher.InputData.Size()
		isData    = common.NewVectorBuilder(hasher.IsData)
		isActive  = common.NewVectorBuilder(hasher.IsActive)
		state     = common.NewVectorBuilder(hasher.State)
		data      = common.NewVectorBuilder(hasher.Data)
		hash      = common.NewVectorBuilder(hasher.Hash)
		finalHash field.Element
	)

	// Writing the first row
	isData.PushOne()
	isActive.PushOne()
	state.PushZero()
	data.PushField(hasher.InputData.GetColAssignmentAt(run, 0))
	hash.PushField(mimc.BlockCompression(state.Last(), data.Last()))

	// Writing the second row
	isData.PushOne()
	isActive.PushOne()
	state.PushField(hash.Last())
	data.PushField(hasher.InputData.GetColAssignmentAt(run, 1))
	hash.PushField(mimc.BlockCompression(state.Last(), data.Last()))

	for j := 2; j < inputSize; j++ {

		inputIsActive := hasher.InputIsActive.GetColAssignmentAt(run, j)
		if !inputIsActive.IsOne() {
			finHash := hash.Last()
			finalHash.Set(&finHash)
			break
		}

		// Odds rows, we start a new hash from zero, using the previous hash
		// as data.
		isData.PushZero()
		isActive.PushOne()
		state.PushZero()
		data.PushField(hash.Last())
		hash.PushField(mimc.BlockCompression(state.Last(), data.Last()))

		// Evens rows, we continue the hash by adding the input data corresponding
		// to "j".
		isData.PushOne()
		isActive.PushOne()
		state.PushField(hash.Last())
		data.PushField(hasher.InputData.GetColAssignmentAt(run, j))
		hash.PushField(mimc.BlockCompression(state.Last(), data.Last()))
	}

	// assign the hasher columns
	isData.PadAndAssign(run, field.Zero())
	isActive.PadAndAssign(run, field.Zero())
	state.PadAndAssign(run, field.Zero())
	data.PadAndAssign(run, field.Zero())
	hash.PadAndAssign(run, mimc.BlockCompression(field.Zero(), field.Zero()))
	run.AssignColumn(hasher.HashFinal.GetColID(), smartvectors.NewConstant(finalHash, hasher.HashFinal.Size()))

	hasher.IsDataFirstRow.Assign(run)
	hasher.IsDataOddRows.Assign(run)
}
