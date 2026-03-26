package importpad

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// It prints the Importation columns, it is not safe to be used directly for test.
// Rather the csv file for Importation columns should be filled up manually as the expected values.
// The function is kept here for reproducibility.
//
//lint:ignore U1000 Ignore unused function temporarily for debugging
func generateImportation(run *wizard.ProverRuntime, mod Importation, path string) {

	var (
		limbs      = mod.Limbs.GetColAssignment(run).IntoRegVecSaveAlloc()
		nbytes     = mod.NBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		hashNum    = mod.HashNum.GetColAssignment(run).IntoRegVecSaveAlloc()
		index      = mod.Index.GetColAssignment(run).IntoRegVecSaveAlloc()
		isActive   = mod.IsActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		isInserted = mod.IsInserted.GetColAssignment(run).IntoRegVecSaveAlloc()
		isPadded   = mod.IsPadded.GetColAssignment(run).IntoRegVecSaveAlloc()
		acc        = mod.AccPaddedBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		isNewHash  = mod.IsNewHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		oF         = files.MustOverwrite(path)
	)

	fmt.Fprintf(oF, "%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
		string(mod.HashNum.GetColID()),
		string(mod.Index.GetColID()),
		string(mod.IsActive.GetColID()),
		string(mod.IsInserted.GetColID()),
		string(mod.IsPadded.GetColID()),
		string(mod.IsNewHash.GetColID()),
		string(mod.Limbs.GetColID()),
		string(mod.NBytes.GetColID()),
		string(mod.AccPaddedBytes.GetColID()),
	)

	for i := range limbs {
		fmt.Fprintf(oF, "%v,%v,%v,%v,%v,%v,0x%v,%v,%v\n",
			hashNum[i].String(),
			index[i].String(),
			isActive[i].String(),
			isInserted[i].String(),
			isPadded[i].String(),
			isNewHash[i].String(),
			limbs[i].Text(16),
			nbytes[i].String(),
			acc[i].String(),
		)
	}

	oF.Close()

}
