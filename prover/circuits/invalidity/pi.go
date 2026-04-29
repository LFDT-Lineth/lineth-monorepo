package invalidity

import (
	"github.com/consensys/gnark/frontend"
	gmimc "github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	linTypesk "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_bls12377"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
)

// FunctionalPublicInputsGnark represents the gnark version of [public_input.Invalidity]
type FunctionalPublicInputsGnark struct {
	FunctionalPIQGnark  `gnark:"-"` // derived from subcircuit in Define, no wires allocated
	TxNumber            frontend.Variable
	DeadLineBlockNumber frontend.Variable // deadline block number to execute/attempt ftx.
	FtxRollingHash      frontend.Variable // 32 bytes from mimc_bls12377

}

// it stores all the fields that should match the FPI of the sub-circuit.
type FunctionalPIQGnark struct {
	TxHash        [2]frontend.Variable // keccak hash needs 2 field elements (16 bytes each)
	FromAddress   frontend.Variable
	StateRootHash [2]frontend.Variable // KoalaBear octuplet converted to 2 BLS12-377 field elements (16 bytes each)
	ToAddress     frontend.Variable    // the to address of the transaction
	// The following fields are used for the extraction of the filtered addresses
	ToIsFiltered   frontend.Variable // 1 if the to address is filtered, 0 otherwise
	FromIsFiltered frontend.Variable // 1 if the from address is filtered, 0 otherwise

	// From execution PI extractor (shared between execution and invalidity)
	CoinBase                frontend.Variable
	BaseFee                 frontend.Variable
	ChainID                 frontend.Variable
	L2MessageServiceAddr    frontend.Variable
	SimulatedBlockTimestamp frontend.Variable // expected to be the initial block timestamp in the aggregation
	SimulatedBlockNumber    frontend.Variable // expected to be the initial block number in the aggregation
}

// Assign the functional public inputs
func (gpi *FunctionalPublicInputsGnark) Assign(pi public_input.Invalidity) {
	gpi.TxHash[0] = pi.TxHash[:16]
	gpi.TxHash[1] = pi.TxHash[16:]
	gpi.FromAddress = pi.FromAddress[:]
	gpi.DeadLineBlockNumber = pi.DeadLineBlockNumber
	gpi.TxNumber = pi.TxNumber

	// Convert octuplet to 32 bytes, then split into two 16-byte chunks
	stateRootBytes := pi.StateRootHash.ToBytes()
	gpi.StateRootHash[0] = stateRootBytes[:16]
	gpi.StateRootHash[1] = stateRootBytes[16:]

	gpi.FtxRollingHash = pi.FtxRollingHash[:]

	// Filtered address fields
	if pi.FromIsFiltered {
		gpi.FromIsFiltered = 1
	} else {
		gpi.FromIsFiltered = 0
	}
	if pi.ToIsFiltered {
		gpi.ToIsFiltered = 1
	} else {
		gpi.ToIsFiltered = 0
	}
	gpi.ToAddress = pi.ToAddress[:]

	gpi.CoinBase = pi.CoinBase[:]
	gpi.BaseFee = pi.BaseFee
	gpi.ChainID = pi.ChainID
	gpi.L2MessageServiceAddr = pi.L2MessageServiceAddr[:]
	gpi.SimulatedBlockTimestamp = pi.SimulatedBlockTimestamp
	gpi.SimulatedBlockNumber = pi.SimulatedBlockNumber
}

// Sum computes the hash over the functional inputs using Poseidon2
// The hash is computed over all fields in a specific order matching the native version
func (spi *FunctionalPublicInputsGnark) Sum(api frontend.API) frontend.Variable {

	hsh, err := poseidon2_bls12377.NewGnarkMDHasher(api)
	if err != nil {
		panic(err)
	}

	hsh.Write(
		spi.TxHash[0],
		spi.TxHash[1],
		spi.TxNumber,
		spi.FromAddress,
		spi.DeadLineBlockNumber,
		spi.StateRootHash[0],
		spi.StateRootHash[1],
		spi.FtxRollingHash,
		spi.ToAddress,
		spi.ToIsFiltered,
		spi.FromIsFiltered,
		spi.CoinBase,
		spi.BaseFee,
		spi.ChainID,
		spi.L2MessageServiceAddr,
		spi.SimulatedBlockTimestamp,
		spi.SimulatedBlockNumber,
	)

	return hsh.Sum()
}

// RangeCheck constrains the functional public inputs to their expected bit widths.
//   - TxHash, FromAddress, StateRootHash — constrained by the sub-circuit IOP
//   - FtxRollingHash — natively constrained as a BLS12-377 field element
func (pi *FunctionalPublicInputsGnark) RangeCheck(api frontend.API) {
	rc := rangecheck.New(api)

	// Outer witness fields
	rc.Check(pi.TxNumber, 64)
	rc.Check(pi.DeadLineBlockNumber, 64)

	// Sub-circuit-derived fields: only boolean-constrained in filtered_address;
	// must be asserted here so the constraint applies to all sub-circuit variants.
	api.AssertIsBoolean(pi.FunctionalPIQGnark.ToIsFiltered)
	api.AssertIsBoolean(pi.FunctionalPIQGnark.FromIsFiltered)

	// Block-level values: unconstrained witnesses in nonce_balance and
	// filtered_address sub-circuits (only IOP-derived in bad_precompile).
	rc.Check(pi.FunctionalPIQGnark.SimulatedBlockTimestamp, 64)
	rc.Check(pi.FunctionalPIQGnark.SimulatedBlockNumber, 64)
}

// UpdateFtxRollingHash updates the ftxRollingHash
func UpdateFtxRollingHash(
	prevFtxRollingHash types.Bls12377Fr,
	txHash common.Hash,
	expectedBlockHeight uint64,
	fromAddress linTypes.EthAddress,
) types.Bls12377Fr {

	hasher := mimc.NewMiMC()

	hasher.Write(prevFtxRollingHash[:])
	hasher.Write(txHash[:LIMB_SIZE])
	hasher.Write(txHash[LIMB_SIZE:])
	linTypesk.WriteInt64On32Bytes(hasher, int64(expectedBlockHeight))
	hasher.Write(fromAddress[:])

	sum := hasher.Sum(nil)
	return types.AsBls12377Fr(sum)
}

// UpdateFtxRollingHash computes the FTX rolling hash in-circuit using MiMC.
// It hashes: prevFtxRollingHash || TxHash[0] || TxHash[1] || ExpectedBlockNumber || FromAddress
func UpdateFtxRollingHashGnark(api frontend.API, in FtxRollingHashInputs) frontend.Variable {
	hsh, err := gmimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}

	hsh.Write(in.PrevFtxRollingHash)
	hsh.Write(in.TxHash0)
	hsh.Write(in.TxHash1)
	hsh.Write(in.ExpectedBlockHeight)
	hsh.Write(in.FromAddress)

	return hsh.Sum()
}

type FtxRollingHashInputs struct {
	PrevFtxRollingHash  frontend.Variable
	TxHash0             frontend.Variable
	TxHash1             frontend.Variable
	ExpectedBlockHeight frontend.Variable
	FromAddress         frontend.Variable
}
