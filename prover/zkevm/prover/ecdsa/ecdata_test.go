package ecdsa

import (
	"os"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/stretchr/testify/require"
)

func TestEcDataAssignData(t *testing.T) {
	f, err := os.Open("testdata/ecdata_test.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal(err)
	}
	limits := &Settings{
		MaxNbEcRecover: 3, // data has three entires, test if can align when have bigger limit
	}
	var ecRec *EcRecover
	var ecSrc *ecDataSource
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			ecSrc = &ecDataSource{
				CsEcrecover: ct.GetCommit(b, "EC_DATA_CS_ECRECOVER"),
				ID:          ct.GetCommit(b, "EC_DATA_ID"),
				SuccessBit:  ct.GetCommit(b, "EC_DATA_SUCCESS_BIT"),
				Index:       ct.GetCommit(b, "EC_DATA_INDEX"),
				IsData:      ct.GetCommit(b, "EC_DATA_IS_DATA"),
				IsRes:       ct.GetCommit(b, "EC_DATA_IS_RES"),
				Limb:        ct.GetLimbsLe(b, "EC_DATA_LIMB", common.NbLimbU128).AssertUint128(),
			}

			ecRec = newEcRecover(b.CompiledIOP, limits, ecSrc)
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {

			ct.Assign(run,
				ecSrc.CsEcrecover,
				ecSrc.ID,
				ecSrc.Limb,
				ecSrc.SuccessBit,
				ecSrc.Index,
				ecSrc.IsData,
				ecSrc.IsRes,
			)

			ecRec.Assign(run, ecSrc)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}

	t.Log("proof succeeded")
}

func TestEcDataAssignData_SkipsNonEcrecoverRows(t *testing.T) {
	ct, err := csvtraces.NewCsvTrace(strings.NewReader(ecdataWithGapCSV))
	require.NoError(t, err)

	limits := &Settings{
		MaxNbEcRecover: 1,
	}

	var ecRec *EcRecover
	var ecSrc *ecDataSource
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			ecSrc = &ecDataSource{
				CsEcrecover: ct.GetCommit(b, "EC_DATA_CS_ECRECOVER"),
				ID:          ct.GetCommit(b, "EC_DATA_ID"),
				SuccessBit:  ct.GetCommit(b, "EC_DATA_SUCCESS_BIT"),
				Index:       ct.GetCommit(b, "EC_DATA_INDEX"),
				IsData:      ct.GetCommit(b, "EC_DATA_IS_DATA"),
				IsRes:       ct.GetCommit(b, "EC_DATA_IS_RES"),
				Limb:        ct.GetLimbsLe(b, "EC_DATA_LIMB", common.NbLimbU128).AssertUint128(),
			}

			ecRec = newEcRecover(b.CompiledIOP, limits, ecSrc)
		},
		dummy.Compile,
	)

	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {
			ct.Assign(run,
				ecSrc.CsEcrecover,
				ecSrc.ID,
				ecSrc.Limb,
				ecSrc.SuccessBit,
				ecSrc.Index,
				ecSrc.IsData,
				ecSrc.IsRes,
			)

			require.NotPanics(t, func() {
				ecRec.Assign(run, ecSrc)
			}, "non-ecrecover source rows should not consume antichamber capacity")
		})

	require.NoError(t, wizard.Verify(cmp, proof))
}

const ecdataWithGapCSV = "EC_DATA_CS_ECRECOVER,EC_DATA_ID,EC_DATA_LIMB," +
	"EC_DATA_SUCCESS_BIT,EC_DATA_INDEX,EC_DATA_IS_DATA,EC_DATA_IS_RES\n" +
	`0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
0,0,0x0,0,0,0,0
1,1,0x279d94621558f755796898fc4bd36b6d,1,0,1,0
1,1,0x407cae77537865afe523b79c74cc680b,1,1,1,0
1,1,0x0,1,2,1,0
1,1,0x1b,1,3,1,0
1,1,0xc2ff96feed8749a5ad1c0714f950b5ac,1,4,1,0
1,1,0x939d8acedbedcbc2949614ab8af06312,1,5,1,0
1,1,0x1feecd50adc6273fdd5d11c6da18c8cf,1,6,1,0
1,1,0xe14e2787f5a90af7c7c1328e7d0a2c42,1,7,1,0
1,1,0xb2e17f39,1,0,0,1
1,1,0xd52e04340f4041e4ceb2b02884406893,1,1,0,1
`
