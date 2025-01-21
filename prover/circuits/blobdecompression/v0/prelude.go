package v0

import (
	"errors"
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v0/compress/lzss"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

type builder struct {
	dict []byte
}

func NewBuilder(dict []byte) *builder {
	return &builder{dict: dict}
}

func (b *builder) Compile() (constraint.ConstraintSystem, error) {
	return MakeCS(b.dict), nil
}

// builds the circuit
func MakeCS(dict []byte) constraint.ConstraintSystem {
	circuit := Allocate(dict)

	scs, err := frontend.Compile(fr.Modulus(), scs.NewBuilder, &circuit, frontend.WithCapacity(1<<27))
	if err != nil {
		panic(err)
	}
	logrus.Infof("successfully compiled the outer-circuit, has %v constraints", scs.GetNbConstraints())
	return scs
}

// Assign the circuit with concrete data. Returns the assigned circuit and the
// public input computed during the assignment.
// @alexandre.belling should we instead compute snarkHash independently here? Seems like it doesn't need to be included in the req received by Prove
func Assign(blobData []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (assignment frontend.Circuit, publicInput fr.Element, snarkHash []byte, err error) {
	const maxCLen = blob.MaxUsableBytes
	const maxDLen = blob.MaxUncompressedBytes

	blobDataUnpacked, err := blob.UnpackAlign(blobData)
	if err != nil {
		err = fmt.Errorf("decompression circuit assignment : could not unpack the data : %w", err)
		return
	}

	header, uncompressedData, _, err := blob.DecompressBlob(blobData, dictStore)
	if err != nil {
		err = fmt.Errorf("decompression circuit assignment : could not decompress the data : %w", err)
		return
	}

	blobDataVar, err := assignVarByteSlice(blobData, maxCLen)
	if err != nil {
		err = fmt.Errorf("decompression circuit assignment : casting the compressed data into frontend.Variable : %w", err)
		return
	}

	// Recomputes the SNARK hash from the blob instead of taking the provided one
	// @alex: this is only needed for the 4844 migration. After that, we are good.
	if snarkHash, err = computeSnarkHash(blobData); err != nil {
		utils.Panic("could not recompute the shnarf: %v", err.Error())
	}

	cPacked, err1 := packBytesInWords(blobData, maxCLen)
	dPacked, err2 := packBytesInWords(uncompressedData, maxDLen)
	if err1 != nil || err2 != nil {
		// the errors have been already checked
		panic(errors.Join(err1, err2))
	}

	cSum := mimcHash(cPacked...)
	dSum := mimcHash(dPacked...)

	var xReduced fr381.Element
	xReduced.SetBytes(x[:])

	input := mimcHashAny(
		cSum,
		dSum,
		xReduced,
		y,
		snarkHash,
		boolToVar(eip4844Enabled),
	)

	// @alex: it's important to register the hints for the solver later. We do
	// it in the assign's function because it would not make sense to either
	// delegate this to the caller (it's a complexity leak) or to the plonkutil
	// package (because its scope is wider than just compression). So, we do it
	// in the Assign function although this creates an unintuitive side effect.
	// It should be harmless though.
	lzss.RegisterHints()
	//internal.RegisterHints()
	utils.RegisterHints()

	a := Circuit{
		BlobBytesLen:       len(blobDataUnpacked),
		BlobPackedBytes:    blobDataVar,
		PublicInput:        input,
		BlobHeaderBytesLen: header.ByteSize(),
		SnarkHash:          snarkHash,
		Eip4844Enabled:     boolToVar(eip4844Enabled),
	}

	if x377, _err := internal.Bls12381ScalarToBls12377Scalars(&xReduced); _err != nil {
		err = fmt.Errorf("decompression circuit assignment : could not convert the x scalar : %w", _err)
		return
	} else {
		a.X[0], a.X[1] = x377[0], x377[1]
	}

	if y377, _err := internal.Bls12381ScalarToBls12377Scalars(y); err != nil {
		err = fmt.Errorf("decompression circuit assignment : could not convert the y scalar : %w", _err)
		return
	} else {
		a.Y[0], a.Y[1] = y377[0], y377[1]
	}

	assignment = &a

	return
}

func boolToVar(b bool) frontend.Variable {
	res := frontend.Variable(0)
	if b {
		res = 1
	}
	return res
}

// Computes the SNARK hash of a stream of byte. Returns the hex string. The hash
// can fail if the input stream does not have the right format.
func computeSnarkHash(stream []byte) ([]byte, error) {
	const blobBytes = 32 * 4096

	h := mimc.NewMiMC()

	if len(stream) > blobBytes {
		return nil, fmt.Errorf("the compressed blob is too large : %v bytes, the limit is %v bytes", len(stream), blobBytes)
	}

	if _, err := h.Write(stream); err != nil {
		return nil, fmt.Errorf("cannot generate Snarkhash of the string `%x`, MiMC failed : %w", stream, err)
	}

	// @alex: for consistency with the circuit, we need to hash the whole input
	// stream padded.
	if len(stream) < blobBytes {
		h.Write(make([]byte, blobBytes-len(stream)))
	}
	return h.Sum(nil), nil
}
