package ecdsa

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/gnark-crypto/ecc/secp256k1/ecdsa"
	fr_secp256k1 "github.com/consensys/gnark-crypto/ecc/secp256k1/fr"
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic/testdata"
	"golang.org/x/crypto/sha3"
)

func TestAntichamber(t *testing.T) {
	f, err := os.Open("testdata/antichamber.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal(err)
	}
	var ac *antichamber
	var ecSrc *ecDataSource
	var txSrc *txnData
	limits := &Settings{
		MaxNbEcRecover:     4,
		MaxNbTx:            2,
		NbInputInstance:    6,
		NbCircuitInstances: 1,
	}
	var rlpTxn generic.GenDataModule

	// to cover edge-cases, leaves some rows empty
	c := testCaseAntiChamber
	// random value for testing edge cases
	nbRowsPerTxInTxnData := 3
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			comp := b.CompiledIOP
			// declare rlp_txn module
			rlpTxn = testdata.CreateGenDataModule(comp, "TXN_RLP", 32, common.NbLimbU128)

			// declar txn_data module
			txSrc = commitTxnData(comp, limits, nbRowsPerTxInTxnData)

			// declare ecdata (ecRecover)
			ecSrc = &ecDataSource{
				CsEcrecover: ct.GetCommit(b, "EC_DATA_CS_ECRECOVER"),
				ID:          ct.GetCommit(b, "EC_DATA_ID"),
				SuccessBit:  ct.GetCommit(b, "EC_DATA_SUCCESS_BIT"),
				Index:       ct.GetCommit(b, "EC_DATA_INDEX"),
				IsData:      ct.GetCommit(b, "EC_DATA_IS_DATA"),
				IsRes:       ct.GetCommit(b, "EC_DATA_IS_RES"),
				Limb:        ct.GetLimbsLe(b, "EC_DATA_LIMB", common.NbLimbU128).AssertUint128(),
			}

			ac = newAntichamber(
				b.CompiledIOP,
				&antichamberInput{
					EcSource:     ecSrc,
					TxSource:     txSrc,
					RlpTxn:       rlpTxn,
					PlonkOptions: []query.PlonkOption{query.PlonkRangeCheckOption(16, 1, true)},
					Settings:     limits,
					WithCircuit:  true,
				},
			)
		},
		dummy.Compile,
	)
	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {

			// assign data to rlp_txn module
			testdata.GenerateAndAssignGenDataModule(run, &rlpTxn, c.HashNum, c.ToHash, true)
			trace := keccak.GenerateTrace(rlpTxn.ScanStreams(run))

			// assign txn_data module from pk
			txSrc.assignTxnDataFromPK(run, ac, trace.HashOutPut, nbRowsPerTxInTxnData)

			ct.Assign(run,
				ecSrc.CsEcrecover,
				ecSrc.ID,
				ecSrc.Limb,
				ecSrc.SuccessBit,
				ecSrc.Index,
				ecSrc.IsData,
				ecSrc.IsRes,
			)
			ac.assign(run, dummyTxSignatureGetter, limits.MaxNbTx)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
}

func dummyTxSignatureGetter(i int, txHash []byte) (r, s, v *big.Int, err error) {
	// some dummy values from the traces
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
	// if not found, create a random signature (which results in random public key)
	_, r, s, v, err = generateDeterministicSignature(txHash)
	if err != nil {
		return nil, nil, nil, err
	}

	return r, s, v, nil
}

func generateDeterministicSignature(txHash []byte) (pk *ecdsa.PublicKey, r, s, v *big.Int, err error) {
	reader := sha3.NewShake128()
	reader.Write(txHash)
	// heuristic number of loops. We use deterministic generation and know the bounds
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
			// try again if s is too big.
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

var testCaseAntiChamber = makeTestCase{
	HashNum: []int{1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2},
	ToHash:  []int{1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1, 1},
}
