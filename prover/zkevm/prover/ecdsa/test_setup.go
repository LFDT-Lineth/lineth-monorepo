package ecdsa

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"

	"github.com/consensys/gnark-crypto/ecc/secp256k1/ecdsa"
	fr_secp256k1 "github.com/consensys/gnark-crypto/ecc/secp256k1/fr"
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic/testdata"
	"golang.org/x/crypto/sha3"
)

// TestEcdsaSetup holds a compiled ECDSA module and prover function for use in
// integration tests from other packages (e.g. invalidity PI).
type TestEcdsaSetup struct {
	KeccakTrace keccak.PermTraces
	FromAddress types.EthAddress // from address of the first transaction
	Ecdsa       *EcdsaZkEvm
	Settings    *Settings
	ProverFunc  func(run *wizard.ProverRuntime)
}

// NewTestEcdsaSetup creates a real ECDSA antichamber module using the test
// data from testdata/antichamber.csv. maxNbTx controls how many transactions
// are included. The caller must invoke this within a wizard.Compile define
// function and call ProverFunc in wizard.Prove.
// The test guarantee that the transaction hash and from address are correctly retrieved from invalidity public inputs as they were assigned in ecdsa.
func NewTestEcdsaSetup(b *wizard.Builder, maxNbTx int) *TestEcdsaSetup {
	comp := b.CompiledIOP

	_, thisFile, _, _ := runtime.Caller(0)
	csvPath := filepath.Join(filepath.Dir(thisFile), "testdata", "antichamber.csv")
	f, err := os.Open(csvPath)
	if err != nil {
		panic("failed to open antichamber.csv: " + err.Error())
	}

	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		f.Close()
		panic("failed to parse antichamber.csv: " + err.Error())
	}
	f.Close()

	var hashNum, toHash []int
	switch maxNbTx {
	case 1:
		hashNum = []int{1, 1, 1, 1, 1, 1}
		toHash = []int{1, 1, 1, 0, 0, 0}
	default:
		hashNum = []int{1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2}
		toHash = []int{1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1, 1}
	}

	limits := &Settings{
		MaxNbEcRecover:     4,
		MaxNbTx:            maxNbTx,
		NbInputInstance:    6,
		NbCircuitInstances: 1,
	}

	nbRowsPerTxInTxnData := 3

	rlpTxn := testdata.CreateGenDataModule(comp, "TXN_RLP", 32, common.NbLimbU128)
	txSrc := commitTxnData(comp, limits, nbRowsPerTxInTxnData)

	ecSrc := &ecDataSource{
		CsEcrecover: ct.GetCommit(b, "EC_DATA_CS_ECRECOVER"),
		ID:          ct.GetCommit(b, "EC_DATA_ID"),
		SuccessBit:  ct.GetCommit(b, "EC_DATA_SUCCESS_BIT"),
		Index:       ct.GetCommit(b, "EC_DATA_INDEX"),
		IsData:      ct.GetCommit(b, "EC_DATA_IS_DATA"),
		IsRes:       ct.GetCommit(b, "EC_DATA_IS_RES"),
		Limb:        ct.GetLimbsLe(b, "EC_DATA_LIMB", common.NbLimbU128).AssertUint128(),
	}

	ac := newAntichamber(
		comp,
		&antichamberInput{
			EcSource:     ecSrc,
			TxSource:     txSrc,
			RlpTxn:       rlpTxn,
			PlonkOptions: []query.PlonkOption{query.PlonkRangeCheckOption(16, 1, true)},
			Settings:     limits,
			WithCircuit:  true,
		},
	)

	ecdsaZkevm := &EcdsaZkEvm{Ant: ac}

	setup := &TestEcdsaSetup{
		Ecdsa:    ecdsaZkevm,
		Settings: limits,
	}

	setup.ProverFunc = func(run *wizard.ProverRuntime) {
		testdata.GenerateAndAssignGenDataModule(run, &rlpTxn, hashNum, toHash, true)
		setup.KeccakTrace = keccak.GenerateTrace(rlpTxn.ScanStreams(run))
		txSrc.assignTxnDataFromPK(run, ac, setup.KeccakTrace.HashOutPut, nbRowsPerTxInTxnData)
		ct.Assign(run,
			ecSrc.CsEcrecover,
			ecSrc.ID,
			ecSrc.Limb,
			ecSrc.SuccessBit,
			ecSrc.Index,
			ecSrc.IsData,
			ecSrc.IsRes,
		)
		hasher := sha3.NewLegacyKeccak256()
		pk, _, _, _, err := generateDeterministicSignature(setup.KeccakTrace.HashOutPut[0][:])
		if err != nil {
			utils.Panic("error generating signature")
		}
		buf := pk.A.RawBytes()

		hasher.Write(buf[:])
		res := hasher.Sum(nil)
		hasher.Reset()

		setup.FromAddress = types.EthAddress(res[12:])
		ac.assign(run, DummyTxSignatureGetter, limits.MaxNbTx)
	}

	return setup
}

// DummyTxSignatureGetter provides r, s, v signature values for known test tx
// hashes from antichamber.csv; falls back to deterministic generation for unknown hashes.
func DummyTxSignatureGetter(i int, txHash []byte) (r, s, v *big.Int, err error) {
	m := map[[32]byte]struct{ r, s, v string }{
		{0x27, 0x9d, 0x94, 0x62, 0x15, 0x58, 0xf7, 0x55, 0x79, 0x68, 0x98, 0xfc, 0x4b, 0xd3, 0x6b, 0x6d, 0x40, 0x7c, 0xae, 0x77, 0x53, 0x78, 0x65, 0xaf, 0xe5, 0x23, 0xb7, 0x9c, 0x74, 0xcc, 0x68, 0xb}: {
			r: "c2ff96feed8749a5ad1c0714f950b5ac939d8acedbedcbc2949614ab8af06312",
			s: "1feecd50adc6273fdd5d11c6da18c8cfe14e2787f5a90af7c7c1328e7d0a2c42",
			v: "1b",
		},
		{0x4b, 0xe1, 0x46, 0xe0, 0x6c, 0xc1, 0xb3, 0x73, 0x42, 0xb6, 0xb7, 0xb1, 0xfa, 0x85, 0x42, 0xae, 0x58, 0xa6, 0x21, 0x3, 0xb8, 0xaf, 0xf, 0x7d, 0x58, 0xf8, 0xa1, 0xff, 0xff, 0xcf, 0x79, 0x14}: {
			r: "a7b0f504b652b3a621921c78c587fdf80a3ab590e22c304b0b0930e90c4e081d",
			s: "5428459ef7e6bd079fbbb7c6fd95cc6c7fe68c93ed4ae75cee36810e79e8a0e5",
			v: "1b",
		},
		{0xca, 0x3e, 0x75, 0x57, 0xa, 0xea, 0xe, 0x3d, 0xd8, 0xe7, 0xa9, 0xd3, 0x8c, 0x2e, 0xfa, 0x86, 0x6f, 0x5e, 0xe2, 0xb1, 0x8b, 0xf5, 0x27, 0xa0, 0xf4, 0xe3, 0x24, 0x8b, 0x7c, 0x7c, 0xf3, 0x76}: {
			r: "f1136900c2cd16eacc676f2c7b70f3dfec13fd16a426aab4eda5d8047c30a9e9",
			s: "4dad8f009ebe31bdc38133bc5fa60e9dca59d0366bd90e2ef12b465982c696aa",
			v: "1c",
		},
	}
	var txHashA [32]byte
	copy(txHashA[:], txHash)
	if v, ok := m[txHashA]; ok {
		r, ok = new(big.Int).SetString(v.r, 16)
		if !ok {
			utils.Panic("failed to parse r")
		}
		s, ok = new(big.Int).SetString(v.s, 16)
		if !ok {
			utils.Panic("failed to parse s")
		}
		vv, ok := new(big.Int).SetString(v.v, 16)
		if !ok {
			utils.Panic("failed to parse v")
		}
		return r, s, vv, nil
	}
	_, r, s, v, err = generateDeterministicSignature(txHash)
	if err != nil {
		return nil, nil, nil, err
	}
	return r, s, v, nil
}

func generateDeterministicSignature(txHash []byte) (pk *ecdsa.PublicKey, r, s, v *big.Int, err error) {
	reader := sha3.NewShake128()
	reader.Write(txHash)
	for i := 0; i < 20; i++ {
		r, err := rand.Int(reader, fr_secp256k1.Modulus())
		if err != nil {
			return nil, nil, nil, nil, err
		}
		s, err := rand.Int(reader, fr_secp256k1.Modulus())
		if err != nil {
			return nil, nil, nil, nil, err
		}
		halfFr := new(big.Int).Sub(fr_secp256k1.Modulus(), big.NewInt(1))
		halfFr.Div(halfFr, big.NewInt(2))
		if s.Cmp(halfFr) > 0 {
			continue
		}
		var v uint = 0
		pk = new(ecdsa.PublicKey)
		if err = pk.RecoverFrom(txHash, v, r, s); err == nil {
			return pk, r, s, new(big.Int).SetUint64(uint64(v + 27)), nil
		}
	}
	return nil, nil, nil, nil, fmt.Errorf("failed to generate a valid signature")
}

func (l *Settings) sizeTxnData(nbRowsPerTxInTxnData int) int {
	return utils.NextPowerOfTwo(l.MaxNbTx * nbRowsPerTxInTxnData)
}

func commitTxnData(comp *wizard.CompiledIOP, limits *Settings, nbRowsPerTxInTxnData int) (td *txnData) {
	size := limits.sizeTxnData(nbRowsPerTxInTxnData)
	return &txnData{
		Ct:       comp.InsertCommit(0, ifaces.ColIDf("txn_data.CT"), size, true),
		User:     comp.InsertCommit(0, ifaces.ColIDf("txn_data.USER"), size, true),
		Selector: comp.InsertCommit(0, ifaces.ColIDf("txn_data.SELECTOR"), size, true),
		From:     limbs.NewUint256Le(comp, "txn_data.From", size),
	}
}

func (td *txnData) assignTxnDataFromPK(
	run *wizard.ProverRuntime,
	ac *Antichamber,
	rlpTxnHashes [][32]byte,
	nbRowsPerTxInTxnData int,
) {
	var (
		hasher      = sha3.NewLegacyKeccak256()
		maxNbTx     = ac.Inputs.Settings.MaxNbTx
		txnCt       = common.NewVectorBuilder(td.Ct)
		txnUser     = common.NewVectorBuilder(td.User)
		txnSelector = common.NewVectorBuilder(td.Selector)
		txnFrom     = limbs.NewVectorBuilder(td.From.AsDynSize())
	)

	pkHash := make([][32]byte, 0, len(rlpTxnHashes))
	for i := range rlpTxnHashes {
		pk, _, _, _, err := generateDeterministicSignature(rlpTxnHashes[i][:])
		if err != nil {
			utils.Panic("error generating signature")
		}
		buf := pk.A.RawBytes()

		hasher.Write(buf[:])
		res := hasher.Sum(nil)
		hasher.Reset()
		pkHash = append(pkHash, [32]byte(res))
	}

	for i := 0; i < maxNbTx; i++ {
		for j := 0; j < nbRowsPerTxInTxnData; j++ {
			txnCt.PushInt(j)
			txnUser.PushOne()
			txnSelector.PushOne()
			if i < len(pkHash) {
				txnFrom.PushBytes(pkHash[i][:])
			} else {
				txnFrom.PushZero()
			}
		}
	}

	txnCt.PadAndAssign(run)
	txnUser.PadAndAssign(run)
	txnSelector.PadAndAssign(run)
	txnFrom.PadAndAssignZero(run)
}

func commitEcRecTxnData(comp *wizard.CompiledIOP, size1 int, size int, ac *Antichamber) (td *txnData, ecRec *EcRecover) {
	td = &txnData{
		Ct:       comp.InsertCommit(0, ifaces.ColIDf("txn_data.CT"), size1, true),
		User:     comp.InsertCommit(0, ifaces.ColIDf("txn_data.USER"), size1, true),
		Selector: comp.InsertCommit(0, ifaces.ColIDf("txn_data.SELECTOR"), size1, true),
		From:     limbs.NewUint256Le(comp, "txn_data.From", size1),
	}

	ecRec = &EcRecover{
		EcRecoverIsRes: comp.InsertCommit(0, ifaces.ColIDf("ECRECOVER_ISRES"), size, true),
		SuccessBit:     comp.InsertCommit(0, ifaces.ColIDf("ECRECOVER_SUCCESSBIT"), size, true),
		Limb:           limbs.NewUint128Le(comp, "ECRECOVER_LIMB", size),
	}

	ac.IsActive = comp.InsertCommit(0, "AntiChamber_IsActive", size, true)
	return td, ecRec
}

func AssignEcRecTxnData(
	run *wizard.ProverRuntime,
	gbm generic.GenDataModule,
	nbEcRec, nbTxS int,
	sizeTxnData, size int,
	td *txnData, ecRec *EcRecover,
	ac *Antichamber,
) {

	var (
		nbRowsPerTxInTxnData = 9

		streams   = gbm.ScanStreams(run)
		permTrace = keccak.GenerateTrace(streams)

		isEcRecRes  = common.NewVectorBuilder(ecRec.EcRecoverIsRes)
		successBit  = common.NewVectorBuilder(ecRec.SuccessBit)
		ecRecLimb   = limbs.NewVectorBuilder(ecRec.Limb.AsDynSize())
		txnCt       = common.NewVectorBuilder(td.Ct)
		txnUser     = common.NewVectorBuilder(td.User)
		txnSelector = common.NewVectorBuilder(td.Selector)
		txnFrom     = limbs.NewVectorBuilder(td.From.AsDynSize())
	)

	if nbEcRec+nbTxS != len(permTrace.HashOutPut) {
		utils.Panic("the number of generated hash %v should be %v + %v", len(permTrace.HashOutPut), nbEcRec, nbTxS)
	}

	for i := 0; i < nbTxS; i++ {
		for j := 0; j < nbRowsPerTxInTxnData; j++ {
			txnCt.PushInt(j)
			txnUser.PushOne()
			txnSelector.PushOne()
			txnFrom.PushBytes(permTrace.HashOutPut[nbEcRec+i][:])
		}
	}

	for i := 0; i < nbEcRec; i++ {

		hashRes := permTrace.HashOutPut[i]

		// Sanity-check that the heights of the builder is the expected one
		if currHeight := i * nbRowsPerEcRec; isEcRecRes.Height() != currHeight || ecRecLimb.Height() != currHeight {
			utils.Panic("isEcRecRes.Height() || ecRecLimb.Height() != %v, %v", isEcRecRes.Height(), ecRecLimb.Height())
		}

		// Pushing the hi part
		isEcRecRes.PushOne()
		successBit.PushOne()
		ecRecLimb.PushLeftPaddedBytes(hashRes[12:16])

		// Pushing the lo part
		isEcRecRes.PushOne()
		successBit.PushOne()
		ecRecLimb.PushBytes(hashRes[16:])

		// Fill the rest of the frame with zeroes
		isEcRecRes.PushSeqOfZeroes(nbRowsPerEcRec - 2)
		successBit.PushSeqOfZeroes(nbRowsPerEcRec - 2)
		ecRecLimb.PushSeqOfZeroes(nbRowsPerEcRec - 2)
	}

	ecRecLimb.PadAndAssignZero(run)
	isEcRecRes.PadAndAssign(run)
	successBit.PadAndAssign(run)
	txnCt.PadAndAssign(run)
	txnUser.PadAndAssign(run)
	txnSelector.PadAndAssign(run)
	txnFrom.PadAndAssignZero(run)

	effectiveSize := nbEcRec*nbRowsPerEcRec + nbTxS*nbRowsPerTxSign
	isActive := vector.Repeat(field.One(), effectiveSize)
	run.AssignColumn(ac.IsActive.GetColID(), smartvectors.RightZeroPadded(isActive, size))
}

func (l *Settings) nbKeccakF(nbKeccakFPerTxn int) int {
	return l.MaxNbTx*nbKeccakFPerTxn + l.MaxNbEcRecover
}
