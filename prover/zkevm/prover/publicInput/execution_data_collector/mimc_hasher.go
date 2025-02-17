package execution_data_collector

import (
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/projection"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

type MIMCHasher struct {
	// a typical isActive binary column, provided as an input to the module
	isActive ifaces.Column
	// the data to be hashed, this column is provided as an input to the module
	inputData     ifaces.Column
	inputIsActive ifaces.Column
	data          ifaces.Column
	canBeData     ifaces.Column // 1 1 0 1 0 1 0 1, complete assignment, never stops
	isData        ifaces.Column //isActive * canBeData
	// this column stores the MiMC hashes
	hash ifaces.Column
	// a constant column that stores the last relevant value of the hash
	HashFinal ifaces.Column
	// state is an intermediary column used to enforce the MiMC constraints
	state ifaces.Column
}

func NewMIMCHasher(comp *wizard.CompiledIOP, inputData, inputIsActive ifaces.Column, name string) *MIMCHasher {
	size := 2 * inputData.Size()
	res := &MIMCHasher{
		inputData:     inputData,
		inputIsActive: inputIsActive,
		data:          util.CreateCol(name, "DATA", size, comp),
		isActive:      util.CreateCol(name, "ACTIVE", size, comp),
		hash:          util.CreateCol(name, "HASH", size, comp),
		HashFinal:     util.CreateCol(name, "HASH_FINAL", 1, comp),
		state:         util.CreateCol(name, "STATE", size, comp),
		isData:        util.CreateCol(name, "IS_DATA", size, comp),
		canBeData:     comp.InsertPrecomputed(ifaces.ColIDf("%s_%s", name, "CAN_BE_DATA"), computeCanBeData(size)),
	}
	return res
}

func DefineHashFilterConstraints(comp *wizard.CompiledIOP, hasher *MIMCHasher, name string) {
	// we require that isActive is binary in DefineIndicatorsMustBeBinary
	// require that the isActive filter only contains 1s followed by 0s
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_IS_ACTIVE_CONSTRAINT_NO_0_TO_1", name),
		sym.Sub(
			hasher.isActive,
			sym.Mul(
				column.Shift(hasher.isActive, -1),
				hasher.isActive,
			),
		),
	)
	util.MustBeBinary(comp, hasher.isActive)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_IS_DATA", name),
		sym.Sub(
			hasher.isData,
			sym.Mul(
				hasher.isActive,
				hasher.canBeData,
			),
		),
	)
	util.MustBeBinary(comp, hasher.isData)
}

// DefineHasher defines the constraints of the MIMCHasher.
// Its isActive and data columns are assumed to be already constrained in another module, no need to constrain them again.
func (hasher *MIMCHasher) DefineHasher(comp *wizard.CompiledIOP, name string) {

	// MiMC constraints
	comp.InsertMiMC(0, ifaces.QueryIDf("%s_%s", name, "MIMC_CONSTRAINT"), hasher.data, hasher.state, hasher.hash)

	// intermediary state integrity
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "CONSISTENCY_STATE_AND_HASH_LAST"), // LAST is either hashSecond
		sym.Add(
			sym.Mul(
				hasher.isData,
				sym.Sub(hasher.state,
					column.Shift(hasher.hash, -1),
				),
			),
			sym.Mul(
				sym.Sub(1, hasher.isData),
				sym.Sub(hasher.state,
					0,
				),
			),
		),
	)

	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "CONSISTENCY_STATE_AND_HASH_LAST_2"), // LAST is either hashSecond
		sym.Mul(
			sym.Sub(1, hasher.isData),
			sym.Sub(hasher.data,
				column.Shift(hasher.hash, -1),
			),
		),
	)

	// state, the current state column, is initially zero
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "INTER_LOCAL"), ifaces.ColumnAsVariable(hasher.state))

	// prepare accessors for HashFinal
	comp.Columns.SetStatus(hasher.HashFinal.GetColID(), column.Proof)
	accHashFinal := accessors.NewFromPublicColumn(hasher.HashFinal, 0)
	// constrain HashFinal
	util.CheckLastELemConsistency(comp, hasher.isActive, hasher.hash, accHashFinal, name)

	// constraint isActive
	DefineHashFilterConstraints(comp, hasher, name)

	projection.InsertProjection(comp,
		ifaces.QueryIDf("%s_%s", name, "PROJECTION_DATA"),
		[]ifaces.Column{hasher.data},
		[]ifaces.Column{hasher.inputData},
		hasher.isData,
		hasher.inputIsActive,
	)

}

// AssignHasher assigns the data in the MIMCHasher. The data and isActive columns are fetched from another module.
func (hasher *MIMCHasher) AssignHasher(run *wizard.ProverRuntime) {
	inputSize := hasher.inputData.Size()
	size := hasher.data.Size()
	isData := make([]field.Element, size)
	data := make([]field.Element, size)
	isActive := make([]field.Element, size)
	hash := make([]field.Element, size)
	state := make([]field.Element, size)

	var (
		finalHash field.Element
	)

	isData[0].SetOne()
	isData[1].SetOne()
	isActive[0].SetOne()
	isActive[1].SetOne()

	for j := 2; j < inputSize; j++ {
		index := j*2 - 1 // corresponding index of the MIMC hasher
		inputIsActive := hasher.inputIsActive.GetColAssignmentAt(run, j)
		if inputIsActive.IsOne() {
			isData[index].SetOne()
		}
	}

	// state[0] remains 0
	// compute first hash
	firstData := hasher.inputData.GetColAssignmentAt(run, 0)
	data[0].Set(&firstData)
	firstHash := mimc.BlockCompression(state[0], data[0])
	hash[0].Set(&firstHash)

	// second hash
	secondData := hasher.inputData.GetColAssignmentAt(run, 1)
	data[1].Set(&secondData)
	state[1].Set(&firstHash)
	secondHash := mimc.BlockCompression(state[1], data[1])
	hash[1].Set(&secondHash)

	inputCounter := 2 // the counter for the input data to process
	// start i from 2, which contains unset field elements
	// i is the hasher counter
	for i := 2; i < len(hash); i++ {
		var dataToHash field.Element
		if isData[i].IsOne() {
			dataToHash = hasher.inputData.GetColAssignmentAt(run, inputCounter)
			state[i].Set(&hash[i-1])
		} else {
			// state[i] remains zero
			dataToHash = hash[i-1]
		}
		data[i].Set(&dataToHash)
		mimcOutput := mimc.BlockCompression(state[i], data[i])
		hash[i].Set(&mimcOutput)

		// set the active filters
		if isData[i].IsOne() {
			// if we just hashed concrete data, we set the active filters and the final hash
			inputIsActive := hasher.inputIsActive.GetColAssignmentAt(run, inputCounter)
			if inputIsActive.IsOne() {
				isActive[i].SetOne()
				isActive[i-1].SetOne()
				finalHash.Set(&mimcOutput)
				inputCounter++
			}
		}
	}

	// assign the hasher columns
	run.AssignColumn(hasher.hash.GetColID(), smartvectors.NewRegular(hash))
	run.AssignColumn(hasher.state.GetColID(), smartvectors.NewRegular(state))
	run.AssignColumn(hasher.data.GetColID(), smartvectors.NewRegular(data))
	run.AssignColumn(hasher.isActive.GetColID(), smartvectors.NewRegular(isActive))
	run.AssignColumn(hasher.isData.GetColID(), smartvectors.NewRegular(isData))
	run.AssignColumn(hasher.HashFinal.GetColID(), smartvectors.NewRegular([]field.Element{finalHash}))

}

func computeCanBeData(size int) smartvectors.SmartVector {
	vect := make([]field.Element, size)
	vect[0].SetOne()
	vect[1].SetOne()
	for i := 3; i < len(vect); i += 2 {
		vect[i].SetOne()
	}
	return smartvectors.NewRegular(vect)
}
