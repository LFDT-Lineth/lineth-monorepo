package statesummary

import (
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

/*
KeysAndBlock is used to compute the keys of the map that will point an account address, storage key and its associated block number
to the relevant storage value we want to fetch from the arithmetization.
This will only be used to fetch the final storage values, as it is not needed for the initial ones
*/
type KeysAndBlock struct {
	address    types.Bytes32
	storageKey types.FullBytes32
	block      int
}

/*
ArithmetizationStorageParser will use the prover runtime to inspect the columns of the arithmetization's scp (Storage Consistency Permutation)
it will map storage keys and block numbers to the corresponding storage values in the arithmetization's scp
*/
type ArithmetizationStorageParser struct {
	scp    *HubColumnSet
	run    *wizard.ProverRuntime
	Values map[KeysAndBlock]types.FullBytes32
}

/*
newArithmetizationStorageParser instantiates a new StorageParser for the scp columns in the arithmetization
*/
func newArithmetizationStorageParser(ss *Module, run *wizard.ProverRuntime) *ArithmetizationStorageParser {
	res := &ArithmetizationStorageParser{
		run:    run,
		Values: map[KeysAndBlock]types.FullBytes32{},
	}

	// In case the SCP module is not activated (for instance, for testing). We
	// still need to instantiate the storage parser but we can return one that
	// does not feature the SCP module.
	if ss.ArithmetizationLink != nil {
		res.scp = &ss.ArithmetizationLink.Scp
	}

	return res
}

/*
Process uses the embedded prover runtime object to inspect the columns of the scp
for each address, storage key and block number, it uses the last key occurrence per block (KOC)
to find out the last corresponding storage value present in the arithmetization's scp.
*/
func (sr *ArithmetizationStorageParser) Process() {
	if sr.scp == nil {
		// for testing without using an scp (storage consistency permutation) table
		return
	}
	for index := 0; index < sr.scp.PeekAtStorage.Size(); index++ {
		isLastKOCBlock := sr.scp.LastKOCBlock.GetColAssignmentAt(sr.run, index)
		if isLastKOCBlock.IsOne() {
			blockFieldElem := sr.scp.BlockNumber.GetColAssignmentAt(sr.run, index)
			block := blockFieldElem.Uint64()
			keyHI := sr.scp.KeyHI.GetColAssignmentAt(sr.run, index)
			keyHIBytes := keyHI.Bytes()
			keyLO := sr.scp.KeyLO.GetColAssignmentAt(sr.run, index)
			keyLOBytes := keyLO.Bytes()
			keyBytes := make([]byte, 0, 32)
			keyBytes = append(keyBytes, keyHIBytes[16:]...)
			keyBytes = append(keyBytes, keyLOBytes[16:]...)
			address := sr.scp.Address.GetColAssignmentAt(sr.run, index)
			mapKey := KeysAndBlock{
				address:    address.Bytes(),
				storageKey: types.FullBytes32(keyBytes),
				block:      utils.ToInt(block),
			}

			valueHI := sr.scp.ValueHINext.GetColAssignmentAt(sr.run, index)
			valueHIBytes := valueHI.Bytes()
			valueLO := sr.scp.ValueLONext.GetColAssignmentAt(sr.run, index)
			valueLOBytes := valueLO.Bytes()

			valueBytes := make([]byte, 0, 32)
			valueBytes = append(valueBytes, valueHIBytes[16:]...)
			valueBytes = append(valueBytes, valueLOBytes[16:]...)
			sr.Values[mapKey] = types.FullBytes32(valueBytes)
		}
	}
}
