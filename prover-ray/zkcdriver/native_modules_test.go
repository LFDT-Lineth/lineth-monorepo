package zkcdriver_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

func TestNative(t *testing.T) {
	const (
		zkcPath = "testdata/modexp_"
	)
	cases := []string{"u64", "u128", "u256"}
	for i := range cases {
		t.Run(cases[i], func(t *testing.T) {
			zkcFilePath := zkcPath + cases[i] + ".bin"
			zkcInputPath := zkcPath + cases[i] + ".json"
			if files.CheckFilePath(zkcFilePath) != nil {
				t.Fatalf("zkc file %s does not exist", zkcFilePath)
			}
			if files.CheckFilePath(zkcInputPath) != nil {
				t.Fatalf("zkc input file %s does not exist", zkcInputPath)
			}
			sys := wiop.NewSystemf("zkc-test/native/%s", cases[i])
			sys.NewRound()
			driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, files.MustRead(zkcFilePath))
			inputs := zkcdriver.ReadZkcInputs(zkcInputPath)

			nonnative.Compile(sys)
			rangecheck.Compile(sys)
			lookuptologderivsum.Compile(sys)
			logderivativesum.Compile(sys)
			localvanishing.Compile(sys)
			global.Compile(sys)
			// pcs.Compile(sys)

			proof, pi := sys.Prove(func(rt *wiop.Runtime) {
				driver.AssignWithPreRead(rt, inputs)
			})
			if err := sys.Verify(proof, pi); err != nil {
				t.Fatalf("native test failed: %v", err)
			}
		})
	}
}
