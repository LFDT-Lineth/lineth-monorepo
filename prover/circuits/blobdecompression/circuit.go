package blobdecompression

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	v0 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v0"
	v1 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob"
)

// Compile builds the circuit
func Compile(dictionaryNbBytes int) constraint.ConstraintSystem {
	return v1.Compile(dictionaryNbBytes)
}

// Assign the circuit with concrete data. Returns the assigned circuit and the
// public input computed during the assignment.
func Assign(blobData []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (circuit frontend.Circuit, publicInput fr.Element, snarkHash []byte, err error) {
	vsn := blob.GetVersion(blobData)
	switch vsn {
	case 1:
		return v1.Assign(blobData, dictStore, eip4844Enabled, x, y)
	case 0:
		return v0.Assign(blobData, dictStore, eip4844Enabled, x, y)
	}
	err = fmt.Errorf("decompression circuit assignment : unsupported blob version %d", vsn)
	return
}
