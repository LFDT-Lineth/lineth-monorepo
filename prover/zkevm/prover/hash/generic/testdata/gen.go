package testdata

import (
	"fmt"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

// SplitBytes splits the input slice into subarrays of the provided size.
func SplitBytes(input []byte, limbByteSize int) [][]byte {
	if len(input) == 0 {
		return [][]byte{}
	}

	var result [][]byte
	for i := 0; i < len(input); i += limbByteSize {
		end := i + limbByteSize
		if end > len(input) {
			end = len(input)
		}
		result = append(result, input[i:end])
	}
	return result
}

// it receives columns hashNum and toHash and generates GenDataModule.If flag is true it generate random nBytes between 1 and 16. Otherwise it sets nBytes to 16.
func GenerateAndAssignGenDataModule(run *wizard.ProverRuntime, gdm *generic.GenDataModule,
	hashNumInt, toHashInt []int, flag bool, path ...string) {

	var (
		rng        = rand.New(rand.NewChaCha8([32]byte{}))
		limbCols   = limbs.NewVectorBuilder(gdm.Limbs.AsDynSize())
		nByteCol   = common.NewVectorBuilder(gdm.NBytes)
		hashNumCol = common.NewVectorBuilder(gdm.HashNum)
		toHashCol  = common.NewVectorBuilder(gdm.ToHash)
		indexCol   = common.NewVectorBuilder(gdm.Index)
	)

	for i := range hashNumInt {

		if i == 0 {
			indexCol.PushInt(0)
		} else if hashNumInt[i] != hashNumInt[i-1] {
			indexCol.PushInt(0)
		} else if toHashInt[i] == 0 {
			indexCol.PushIncBy(0)
		} else {
			indexCol.PushIncBy(1)
		}

		toHashCol.PushInt(toHashInt[i])
		hashNumCol.PushInt(hashNumInt[i])
		var numBytesInt int
		if flag {
			numBytesInt = int(rng.Int32N(16) + 1)
			nByteCol.PushInt(numBytesInt)
		} else {
			numBytesInt = 16
			nByteCol.PushInt(16)
		}

		resBytes := make([]byte, 16)
		_, _ = utils.ReadPseudoRand(rng, resBytes[:numBytesInt])
		limbCols.PushBytes(resBytes)
	}

	nByteCol.PadAndAssign(run)
	hashNumCol.PadAndAssign(run)
	indexCol.PadAndAssign(run)
	toHashCol.PadAndAssign(run)
	limbCols.PadAndAssignZero(run)

	if len(path) > 0 {

		oF := files.MustOverwrite(path[0])
		fmt.Fprint(oF, "TO_HASH,HASH_NUM,INDEX,NBYTES,LIMBS\n")

		for i := range hashNumInt {

			limbBytes := limbCols.PeekBytesAt(i)
			limbsStr := fmt.Sprintf("0x%x", limbBytes)

			fmt.Fprintf(oF, "%v,%v,%v,%v,0x%v\n",
				toHashCol.Slice()[i].String(),
				hashNumCol.Slice()[i].String(),
				indexCol.Slice()[i].String(),
				nByteCol.Slice()[i].String(),
				limbsStr,
			)
		}

		oF.Close()
	}

}

// CreateGenDataModule is used for testing, it commits to the [generic.GenDataModule] columns,
func CreateGenDataModule(comp *wizard.CompiledIOP, name string, size int, nbLimbs int) (gbm generic.GenDataModule) {
	createCol := common.CreateColFn(comp, name, size, pragmas.RightPadded)
	return generic.GenDataModule{
		Limbs:   limbs.NewUint128Be(comp, ifaces.ColID(name)+"_LIMBS", size),
		HashNum: createCol("HASH_NUM"),
		Index:   createCol("INDEX"),
		NBytes:  createCol("NBYTES"),
		ToHash:  createCol("TO_HASH"),
	}
}

// CreateGenInfoModule is used for testing, it commits to the [generic.GenInfoModule] columns,
func CreateGenInfoModule(comp *wizard.CompiledIOP, name string, size int, nbLimbs int) (gim generic.GenInfoModule) {
	createCol := common.CreateColFn(comp, name, size, pragmas.RightPadded)
	return generic.GenInfoModule{
		HashHi:   limbs.NewUint128Be(comp, ifaces.ColID(name)+"_HASH_HI", size),
		HashLo:   limbs.NewUint128Be(comp, ifaces.ColID(name)+"_HASH_LO", size),
		IsHashHi: createCol("IS_HASH_HI"),
		IsHashLo: createCol("IS_HASH_LO"),
	}
}

// it embeds  the expected hash (for the steam encoded inside gdm) inside gim columns.
func GenerateAndAssignGenInfoModule(
	run *wizard.ProverRuntime,
	gim *generic.GenInfoModule,
	gdm generic.GenDataModule,
	isHashHi, isHashLo []int,
) {

	var (
		hashHi      = limbs.NewVectorBuilder(gim.HashHi.AsDynSize())
		hashLo      = limbs.NewVectorBuilder(gim.HashLo.AsDynSize())
		isHashHiCol = common.NewVectorBuilder(gim.IsHashHi)
		isHashLoCol = common.NewVectorBuilder(gim.IsHashLo)
	)

	// This loop generates the different streams
	streams := gdm.ScanStreams(run)
	var res [][32]byte
	for _, stream := range streams {
		res = append(res, keccak.Hash(stream))

	}

	ctrHi := 0
	ctrLo := 0
	for i := range isHashHi {

		if isHashHi[i] == 1 {
			hiBytes := res[ctrHi][:16]
			isHashHiCol.PushInt(1)
			hashHi.PushBytes(hiBytes)
			ctrHi++
		} else {
			hashHi.PushZero()
			isHashHiCol.PushZero()
		}

		if isHashLo[i] == 1 {
			loBytes := res[ctrLo][16:]
			isHashLoCol.PushInt(1)
			hashLo.PushBytes(loBytes)
			ctrLo++
		} else {
			isHashLoCol.PushZero()
			hashLo.PushZero()
		}
	}

	hashHi.PadAndAssignZero(run)
	hashLo.PadAndAssignZero(run)
	isHashHiCol.PadAndAssign(run)
	isHashLoCol.PadAndAssign(run)
}
