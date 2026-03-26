package importpad

import (
	"encoding/hex"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/crypto/sha2"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/stretchr/testify/assert"
)

func TestImportAndPad(t *testing.T) {
	var testCases = []struct {
		Name        string
		ModFilePath string
		UseCase     generic.HashingUsecase
		PaddingFunc func(stream []byte) []byte
	}{
		{
			Name:        "Keccak",
			ModFilePath: "testdata/mod_keccak.csv",
			UseCase:     generic.KeccakUsecase,
			PaddingFunc: keccak.PadStream,
		},
		{
			Name:        "Sha2",
			ModFilePath: "testdata/mod_sha2.csv",
			UseCase:     generic.Sha2Usecase,
			PaddingFunc: sha2.PadStream,
		},
	}

	for _, uc := range testCases {
		t.Run(uc.Name, func(t *testing.T) {

			var (
				inp   ImportAndPadInputs
				mod   *Importation
				inpCt = csvtraces.MustOpenCsvFile("testdata/input.csv")
				modCt = csvtraces.MustOpenCsvFile(uc.ModFilePath)
			)

			comp := wizard.Compile(func(build *wizard.Builder) {

				inp = ImportAndPadInputs{
					Name: "TESTING",
					Src: generic.GenericByteModule{Data: generic.GenDataModule{
						HashNum: inpCt.GetCommit(build, "HASH_NUM"),
						Index:   inpCt.GetCommit(build, "INDEX"),
						ToHash:  inpCt.GetCommit(build, "TO_HASH"),
						NBytes:  inpCt.GetCommit(build, "NBYTES"),
						Limbs:   inpCt.GetLimbsBe(build, "LIMBS", common.NbLimbU128).AssertUint128(),
					}},
					PaddingStrategy: uc.UseCase,
				}

				mod = ImportAndPad(build.CompiledIOP, inp, 64)

			}, dummy.Compile)

			proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {

				inpCt.Assign(run,
					inp.Src.Data.Limbs,
					inp.Src.Data.HashNum,
					inp.Src.Data.Index,
					inp.Src.Data.ToHash,
					inp.Src.Data.NBytes,
				)

				mod.Run(run)

				modCt.CheckAssignment(run,
					mod.HashNum,
					mod.Index,
					mod.IsActive,
					mod.IsInserted,
					mod.IsPadded,
					mod.NBytes,
					mod.AccPaddedBytes,
				).CheckAssignmentCols(run,
					mod.Limbs...,
				)

				if uc.PaddingFunc != nil {
					checkPaddingAssignment(t, run, uc.PaddingFunc, mod)
				}
			})

			if err := wizard.Verify(comp, proof); err != nil {
				t.Fatal("proof failed", err)
			}

			t.Log("proof succeeded")

		})
	}
}

func checkPaddingAssignment(t *testing.T, run *wizard.ProverRuntime, paddingFunc func([]byte) []byte, mod *Importation) {

	var (
		paddedGdm = &generic.GenDataModule{
			HashNum: mod.HashNum,
			Limbs:   limbs.NewLimbsFromRawUnsafe[limbs.BigEndian]("TESTING_LIMBS", mod.Limbs).AssertUint128(),
			NBytes:  mod.NBytes,
			ToHash:  mod.IsActive,
			Index:   mod.Index,
		}

		inputStreams        = mod.Inputs.Src.Data.ScanStreams(run)
		actualPaddedStreams = paddedGdm.ScanStreams(run)
	)

	for i := range inputStreams {
		expectedPaddedStream := paddingFunc(inputStreams[i])
		assert.Equal(t,
			hex.EncodeToString(expectedPaddedStream),
			hex.EncodeToString(actualPaddedStreams[i]),
		)
	}
}
