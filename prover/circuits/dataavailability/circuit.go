package dataavailability

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"

	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability/config"
	v2 "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob"
)

// Assign the circuit with concrete data. Returns the assigned circuit and the
// public input computed during the assignment.
func Assign(config config.CircuitSizes, blobData []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (circuit frontend.Circuit, publicInput fr.Element, snarkHash []byte, err error) {
	vsn := blob.GetVersion(blobData)
	switch vsn {
	case 2:
		return v2.Assign(config, blobData, dictStore, eip4844Enabled, x, y)
	}
	err = fmt.Errorf("decompression circuit assignment : unsupported blob version %d", vsn)
	return
}
