package public_input

import (
	"hash"

	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	_ "github.com/consensys/gnark-crypto/ecc/bls12-377/fr/poseidon2"
	gchash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
)

// Invalidity represents the functional public inputs for the invalidity circuit
// The mimc hash over functional inputs is set as the public input of the circuit.
type Invalidity struct {
	TxHash              common.Hash // hash of the unsigned transaction
	TxNumber            uint64
	FromAddress         types.EthAddress    // address of the sender
	DeadLineBlockNumber uint64              //  the max expected block number for the transaction to be executed.
	StateRootHash       types.KoalaOctuplet // state-root-hash on which the invalidity is based
	FtxRollingHash      types.Bls12377Fr    // the rolling hash of the forced transaction from mimc_bls12377
	FromIsFiltered      bool                // 1 if the from address is filtered, 0 otherwise
	ToIsFiltered        bool                // 1 if the to address is filtered, 0 otherwise
	ToAddress           types.EthAddress    // address of the recipient

	// From execution PI (shared between execution and invalidity)
	CoinBase                types.EthAddress
	BaseFee                 uint64
	ChainID                 uint64
	L2MessageServiceAddr    types.EthAddress
	SimulatedBlockTimestamp uint64 // expected to be the initial block timestamp in the aggregation
	SimulatedBlockNumber    uint64 // expected to be the initial block number in the aggregation
}

// Sum compute the Poseidon2 hash over the functional public inputs
func (pi *Invalidity) Sum(hsh hash.Hash) []byte {
	if hsh == nil {
		hsh = gchash.POSEIDON2_BLS12_377.New()
	}
	stateRootHash := pi.StateRootHash.ToBytes()
	hsh.Reset()
	_, err := hsh.Write(pi.TxHash[:16]) // TxHash comes from keccak256(unsigned transaction), needs 2 field elements (16 bytes each)
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(pi.TxHash[16:])
	if err != nil {
		panic(err)
	}
	_, err = writeNum(hsh, pi.TxNumber)
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(pi.FromAddress[:])
	if err != nil {
		panic(err)
	}
	_, err = writeNum(hsh, pi.DeadLineBlockNumber)
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(stateRootHash[:16]) // StateRootHash is an octuplet of 8 field elements (31 bits each), needs 2 field elements (16 bytes each)
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(stateRootHash[16:])
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(pi.FtxRollingHash[:])
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(pi.ToAddress[:])
	if err != nil {
		panic(err)
	}
	if pi.ToIsFiltered {
		_, err = writeNum(hsh, 1)
	} else {
		_, err = writeNum(hsh, 0)
	}
	if err != nil {
		panic(err)
	}
	if pi.FromIsFiltered {
		_, err = writeNum(hsh, 1)
	} else {
		_, err = writeNum(hsh, 0)
	}
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(pi.CoinBase[:])
	if err != nil {
		panic(err)
	}
	_, err = writeNum(hsh, pi.BaseFee)
	if err != nil {
		panic(err)
	}
	_, err = writeNum(hsh, pi.ChainID)
	if err != nil {
		panic(err)
	}
	_, err = hsh.Write(pi.L2MessageServiceAddr[:])
	if err != nil {
		panic(err)
	}
	_, err = writeNum(hsh, pi.SimulatedBlockTimestamp)
	if err != nil {
		panic(err)
	}
	_, err = writeNum(hsh, pi.SimulatedBlockNumber)
	if err != nil {
		panic(err)
	}

	return hsh.Sum(nil)
}

func (pi *Invalidity) SumAsField() fr377.Element {
	sumBytes := pi.Sum(nil)
	sum := new(fr377.Element).SetBytes(sumBytes)
	return *sum
}
