package invalidity

import (
	"bytes"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const LIMB_SIZE = 16

type TxPayloadGnark struct {
	ChainID    frontend.Variable
	Nonce      frontend.Variable
	GasTipCap  frontend.Variable // a.k.a. maxPriorityFeePerGas
	GasFeeCap  frontend.Variable // a.k.a. maxFeePerGas
	Gas        frontend.Variable
	To         frontend.Variable `rlp:"nil"` // nil means contract creation
	Value      frontend.Variable
	Data       []frontend.Variable
	AccessList AccessListGnark
}

// AccessList is an EIP-2930 access list.
type AccessListGnark []AccessTupleGnark

// AccessTuple is the element type of an access list.
type AccessTupleGnark struct {
	Address     frontend.Variable   `json:"address"     gencodec:"required"`
	StorageKeys []frontend.Variable `json:"storageKeys" gencodec:"required"`
}

// CheckKeccakConsistency checks the consistency of keccak module against the given input and output.
func CheckKeccakConsistency(api frontend.API, hashInput []frontend.Variable, hashOutput [2]frontend.Variable, keccak *wizard.VerifierCircuit) {

	var (
		radix       = big.NewInt(256)
		ctr         = 0
		limbCol     = keccak.GetColumn(ifaces.ColIDf("TxHash_INVALIDITY_LIMBS"))
		hashHiCol   = keccak.GetColumn(ifaces.ColIDf("TxHash_INVALIDITY_HASH_HI"))
		hashLoCol   = keccak.GetColumn(ifaces.ColIDf("TxHash_INVALIDITY_HASH_LO"))
		isHashHiCol = keccak.GetColumn(ifaces.ColIDf("TxHash_INVALIDITY_IS_HASH_HI"))
		isHashLoCol = keccak.GetColumn(ifaces.ColIDf("TxHash_INVALIDITY_IS_HASH_LO"))
	)

	// check that the input matches the limb column
	if len(limbCol) < len(hashInput)/16+1 {
		utils.Panic("keccak limb column is not large enough to hold the rlp encoding")
	}

	// split the input into chunks of LIMB_SIZE (16) bytes
	for len(hashInput) > LIMB_SIZE {
		v := hashInput[:LIMB_SIZE]
		curLimb := compress.ReadNum(api, v, radix)
		api.AssertIsEqual(limbCol[ctr], curLimb)
		ctr++
		hashInput = hashInput[LIMB_SIZE:]
	}
	// handle the last chunk
	if len(hashInput) > 0 {
		v := make([]frontend.Variable, LIMB_SIZE)
		copy(v, hashInput)
		for i := len(hashInput); i < LIMB_SIZE; i++ {
			v[i] = 0
		}
		curLimb := compress.ReadNum(api, v, radix)
		api.AssertIsEqual(limbCol[ctr], curLimb)
	}

	// check that the keccak  hash columns matches the hashOutput
	api.AssertIsEqual(hashHiCol[0], hashOutput[0])
	api.AssertIsEqual(hashLoCol[0], hashOutput[1])

	// check that isHashHi and isHashLo are set to 1
	api.AssertIsEqual(isHashHiCol[0], 1)
	api.AssertIsEqual(isHashLoCol[0], 1)

	// check that the rest of the limb column is padded with zeros
	// note that due to the collision resistance of keccak,
	// this check along side the above checks are enough,
	// and we dont need to check (nByte, hashNum, toHash, index) columns.
	for i := ctr + 1; i < len(limbCol); i++ {
		api.AssertIsEqual(limbCol[i], 0)
	}

}

func CreateGenDataModule(comp *wizard.CompiledIOP, size int) generic.GenDataModule {

	gdm := generic.GenDataModule{
		HashNum: comp.InsertProof(0, "TxHash_INVALIDITY_HASH_NUM", size),
		Index:   comp.InsertProof(0, "TxHash_INVALIDITY_INDEX", size),
		Limb:    comp.InsertProof(0, "TxHash_INVALIDITY_LIMBS", size),
		NBytes:  comp.InsertProof(0, "TxHash_INVALIDITY_NBYTES", size),
		ToHash:  comp.InsertProof(0, "TxHash_INVALIDITY_TO_HASH", size),
	}
	return gdm
}

func CreateGenInfoModule(comp *wizard.CompiledIOP, size int) generic.GenInfoModule {

	gim := generic.GenInfoModule{
		HashHi:   comp.InsertProof(0, "TxHash_INVALIDITY_HASH_HI", size),
		HashLo:   comp.InsertProof(0, "TxHash_INVALIDITY_HASH_LO", size),
		IsHashHi: comp.InsertProof(0, "TxHash_INVALIDITY_IS_HASH_HI", size),
		IsHashLo: comp.InsertProof(0, "TxHash_INVALIDITY_IS_HASH_LO", size),
	}
	return gim
}

func AssignGenDataModule(run *wizard.ProverRuntime, gdm *generic.GenDataModule, tx *types.Transaction) {

	var (
		nByteCol   = common.NewVectorBuilder(gdm.NBytes)
		limbCol    = common.NewVectorBuilder(gdm.Limb)
		hashNumCol = common.NewVectorBuilder(gdm.HashNum)
		toHashCol  = common.NewVectorBuilder(gdm.ToHash)
		indexCol   = common.NewVectorBuilder(gdm.Index)
	)

	// get the rlp encoding of the transaction with type prefix.
	prefixedRlp := ethereum.EncodeTxForSigning(tx)

	// compute the hash of the transaction.
	signer := types.NewLondonSigner(tx.ChainId())
	txHash := signer.Hash(tx)
	// sanity check
	h2 := crypto.Keccak256Hash(prefixedRlp)
	if txHash != h2 {
		panic("preimage mismatch")
	}

	expectedStream := make([]byte, len(prefixedRlp))
	copy(expectedStream, prefixedRlp)

	// split the prefixedRlp into limbs of left-aligned LIMB_SIZE (16) bytes.
	ctrIndex := 0
	for len(prefixedRlp) >= LIMB_SIZE {

		var f field.Element
		f.SetBytes(prefixedRlp[:LIMB_SIZE]) // left-aligned
		limbCol.PushField(f)

		nByteCol.PushInt(LIMB_SIZE)
		hashNumCol.PushInt(1)
		toHashCol.PushInt(1)

		if ctrIndex == 0 {
			indexCol.PushInt(0)
		} else {
			indexCol.PushInc()
		}
		ctrIndex++
		prefixedRlp = prefixedRlp[LIMB_SIZE:]
	}
	if len(prefixedRlp) > 0 {
		b := make([]byte, LIMB_SIZE)
		copy(b, prefixedRlp)

		var f field.Element
		f.SetBytes(b) // left-aligned 32 bytes
		limbCol.PushField(f)

		nByteCol.PushInt(len(prefixedRlp))
		hashNumCol.PushInt(1)
		toHashCol.PushInt(1)

		if ctrIndex == 0 {
			indexCol.PushInt(0)
		} else {
			indexCol.PushInc()
		}
	}

	limbCol.PadAndAssign(run)
	nByteCol.PadAndAssign(run)
	hashNumCol.PadAndAssign(run)
	indexCol.PadAndAssign(run)
	toHashCol.PadAndAssign(run)

	//sanity check
	streams := gdm.ScanStreams(run)
	if !bytes.Equal(streams[0], expectedStream) {
		panic("gdm stream does not match input")
	}

}

func AssignGenInfoModule(run *wizard.ProverRuntime, gim *generic.GenInfoModule, tx *types.Transaction) {

	var (
		hashHi      = common.NewVectorBuilder(gim.HashHi)
		hashLo      = common.NewVectorBuilder(gim.HashLo)
		isHashHiCol = common.NewVectorBuilder(gim.IsHashHi)
		isHashLoCol = common.NewVectorBuilder(gim.IsHashLo)
		signer      = types.NewLondonSigner(tx.ChainId())
		txHash      = signer.Hash(tx)
	)

	if len(txHash.Bytes()) != 32 {
		utils.Panic("tx hash length is not 32 bytes")
	}
	// we only have one hash to fill in the gim columns

	var fHi, fLo field.Element
	fHi.SetBytes(txHash[:LIMB_SIZE])
	fLo.SetBytes(txHash[LIMB_SIZE:])
	hashHi.PushField(fHi)
	hashLo.PushField(fLo)
	isHashHiCol.PushInt(1)
	isHashLoCol.PushInt(1)

	hashHi.PadAndAssign(run)
	hashLo.PadAndAssign(run)
	isHashHiCol.PadAndAssign(run)
	isHashLoCol.PadAndAssign(run)
}

// MakeKeccakCompiledIOP creates a compiled wizard IOP for keccak hashing.
// This is used for circuit setup where we only need the compiled IOP, not the proof.
func MakeKeccakCompiledIOP(maxRlpByteSize int, compilationSuite ...func(*wizard.CompiledIOP)) *wizard.CompiledIOP {
	maxNumKeccakF := maxRlpByteSize/136 + 1 //  136 bytes is the number of bytes absorbed per permutation keccakF.
	colSize := maxRlpByteSize/LIMB_SIZE + 1 // each limb is LIMB_SIZE bytes.
	size := utils.NextPowerOfTwo(colSize)

	gdm := generic.GenDataModule{}
	gim := generic.GenInfoModule{}

	define := func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		gdm = CreateGenDataModule(comp, size)
		gim = CreateGenInfoModule(comp, size)

		inp := keccak.KeccakSingleProviderInput{
			MaxNumKeccakF: maxNumKeccakF,
			Provider: generic.GenericByteModule{
				Data: gdm,
				Info: gim},
		}
		keccak.NewKeccakSingleProvider(comp, inp)
	}

	return wizard.Compile(define, compilationSuite...)
}

func MakeKeccakProofs(tx *types.Transaction, maxRlpByteSize int, compilationSuite ...func(*wizard.CompiledIOP)) (
	comp *wizard.CompiledIOP,
	proof wizard.Proof,
) {
	maxNumKeccakF := maxRlpByteSize/136 + 1 //  136 bytes is the number of bytes absorbed per permutation keccakF.
	colSize := maxRlpByteSize/LIMB_SIZE + 1 // each limb is LIMB_SIZE bytes.
	size := utils.NextPowerOfTwo(colSize)

	mod := &keccak.KeccakSingleProvider{}
	gdm := generic.GenDataModule{}
	gim := generic.GenInfoModule{}

	define := func(builder *wizard.Builder) {
		comp := builder.CompiledIOP
		gdm = CreateGenDataModule(comp, size)
		gim = CreateGenInfoModule(comp, size)

		inp := keccak.KeccakSingleProviderInput{
			MaxNumKeccakF: maxNumKeccakF,
			Provider: generic.GenericByteModule{
				Data: gdm,
				Info: gim},
		}
		mod = keccak.NewKeccakSingleProvider(comp, inp)
	}

	prover := func(run *wizard.ProverRuntime) {

		AssignGenDataModule(run, &gdm, tx)
		// expected hash is embedded inside gim columns.
		AssignGenInfoModule(run, &gim, tx)

		mod.Run(run)
	}

	comp = wizard.Compile(define, compilationSuite...)
	proof = wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)

	if err != nil {
		utils.Panic("verifier failed: %v", err)
	}
	return
}
