package ecdsa

import (
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

func TestUnalignedGnarkDataAssign(t *testing.T) {
	f, err := os.Open("testdata/unaligned_gnark_test.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal(err)
	}
	var uag *UnalignedGnarkData
	var uagSrc *unalignedGnarkDataSource
	cmp := wizard.Compile(func(build *wizard.Builder) {
		uagSrc = &unalignedGnarkDataSource{
			Source:     ct.GetCommit(build, "SOURCE"),
			IsPushing:  ct.GetCommit(build, "IS_PUSHING"),
			IsFetching: ct.GetCommit(build, "IS_FETCHING"),
			IsActive:   ct.GetCommit(build, "IS_ACTIVE"),
			SuccessBit: ct.GetCommit(build, "SUCCESS_BIT"),
			IsData:     ct.GetCommit(build, "IS_DATA"),
			IsRes:      ct.GetCommit(build, "IS_RES"),
			Limb:       ct.GetLimbsLe(build, "LIMB", common.NbLimbU128).AssertUint128(),
			TxHashHi:   ct.GetLimbsLe(build, "TX_HASH_HI", common.NbLimbU128).AssertUint128(),
			TxHashLo:   ct.GetLimbsLe(build, "TX_HASH_LO", common.NbLimbU128).AssertUint128(),
		}

		uag = newUnalignedGnarkData(build.CompiledIOP, ct.LenPadded(), uagSrc)
	}, dummy.Compile)

	proof := wizard.Prove(cmp, func(run *wizard.ProverRuntime) {

		ct.Assign(run,
			uagSrc.Source,
			uagSrc.IsActive,
			uagSrc.IsFetching,
			uagSrc.IsPushing,
			uagSrc.Limb,
			uagSrc.SuccessBit,
			uagSrc.IsData,
			uagSrc.IsRes,
			uagSrc.TxHashHi,
			uagSrc.TxHashLo,
		)

		uag.Assign(run, uagSrc, dummyTxSignatureGetter)

		ct.CheckAssignment(run,
			uag.IsPublicKey,
			uag.GnarkIndex,
			uag.GnarkData,
		)
	})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
	t.Log("proof succeeded")
}
