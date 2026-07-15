package zkcdriver_test

import (
	"bytes"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/files"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

func TestNative(t *testing.T) {
	const (
		zkcPath = "testdata/modexp_"
	)
	cases := []string{"u64", "u128", "u256"}
	for i := range cases {
		t.Run(cases[i], func(t *testing.T) {
			zkcInputPath := zkcPath + cases[i] + ".json"
			binf, err := compileBinaryConstraints(zkcPath + cases[i] + ".zkc")
			if err != nil {
				t.Fatalf("failed to compile zkc source: %v", err)
			}
			if files.CheckFilePath(zkcInputPath) != nil {
				t.Fatalf("zkc input file %s does not exist", zkcInputPath)
			}
			binfBytes, err := binf.MarshalBinary()
			if err != nil {
				t.Fatalf("failed to marshal binary constraints: %v", err)
			}
			sys := wiop.NewSystemf("zkc-test/native/%s", cases[i])
			sys.NewRound()
			driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(binfBytes))
			inputs := zkcdriver.ReadZkcInputs(zkcInputPath)

			proverCompilePipeline(sys)

			proof, pi := sys.Prove(func(rt *wiop.Runtime) {
				driver.AssignWithPreRead(rt, inputs)
			})
			if err := sys.Verify(proof, pi); err != nil {
				t.Fatalf("native test failed: %v", err)
			}
		})
	}
}
