package testdata

import (
	"fmt"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
)

// it receives columns hashNum and toHash and generates GenDataModule.
func GenerateAndAssignGenDataModule(run *wizard.ProverRuntime, gdm *generic.GenDataModule,
	hashNumInt, toHashInt []int, flag bool, path ...string) {

	var (
		size    = gdm.Limb.Size()
		limbs   = make([]field.Element, size)
		nBytes  = make([]field.Element, size)
		toHash  = make([]field.Element, size)
		index   = make([]field.Element, size)
		hashNum = make([]field.Element, size)
		rng     = rand.New(rand.NewChaCha8([32]byte{}))

		nByteCol   = common.NewVectorBuilder(gdm.NBytes)
		limbCol    = common.NewVectorBuilder(gdm.Limb)
		hashNumCol = common.NewVectorBuilder(gdm.HashNum)
		toHashCol  = common.NewVectorBuilder(gdm.ToHash)
		indexCol   = common.NewVectorBuilder(gdm.Index)
	)

	for i := range hashNumInt {

		if i == 0 {
			index[i] = field.Zero()
		} else if hashNumInt[i] != hashNumInt[i-1] {
			index[i] = field.Zero()
		} else if toHashInt[i] == 0 {
			index[i] = index[i-1]
		} else {
			index[i].Add(&index[i-1], new(field.Element).SetOne())
		}

		toHash[i] = field.NewElement(uint64(toHashInt[i]))
		hashNum[i] = field.NewElement(uint64(hashNumInt[i]))
		var numBytesInt int
		var numBytesF field.Element
		if flag {
			numBytesInt, numBytesF = randNBytes(rng)
			nBytes[i] = numBytesF
		} else {
			nBytes[i] = field.NewElement(16)
			numBytesInt = 16
		}

		limbs[i] = randLimbs(rng, numBytesInt)
	}

	limbCol.PushSliceF(limbs)
	nByteCol.PushSliceF(nBytes)
	hashNumCol.PushSliceF(hashNum)
	indexCol.PushSliceF(index)
	toHashCol.PushSliceF(toHash)

	limbCol.PadAndAssign(run)
	nByteCol.PadAndAssign(run)
	hashNumCol.PadAndAssign(run)
	indexCol.PadAndAssign(run)
	toHashCol.PadAndAssign(run)

	if len(path) > 0 {

		oF := files.MustOverwrite(path[0])
		fmt.Fprint(oF, "TO_HASH,HASH_NUM,INDEX,NBYTES,LIMBS\n")

		for i := range hashNumInt {
			fmt.Fprintf(oF, "%v,%v,%v,%v,0x%v\n",
				toHash[i].String(),
				hashNum[i].String(),
				index[i].String(),
				nBytes[i].String(),
				limbs[i].Text(16),
			)
		}

		oF.Close()
	}

}

func randNBytes(rng *rand.Rand) (int, field.Element) {

	// nBytesInt must be in 1..=16
	var (
		nBytesInt = rng.Int32N(16) + 1
		nBytesF   = field.NewElement(uint64(nBytesInt))
	)

	return int(nBytesInt), nBytesF
}

func randLimbs(rng *rand.Rand, nBytes int) field.Element {

	var (
		resBytes = make([]byte, 16)
		_, _     = utils.ReadPseudoRand(rng, resBytes[:nBytes])
		res      = new(field.Element).SetBytes(resBytes)
	)

	return *res
}

// CreateGenDataModule is used for testing, it commits to the [generic.GenDataModule] columns,
func CreateGenDataModule(
	comp *wizard.CompiledIOP,
	name string,
	size int,
) (gbm generic.GenDataModule) {
	createCol := common.CreateColFn(comp, name, size, pragmas.RightPadded)
	gbm.HashNum = createCol("HASH_NUM")
	gbm.Index = createCol("INDEX")
	gbm.Limb = createCol("LIMBS")
	gbm.NBytes = createCol("NBYTES")
	gbm.ToHash = createCol("TO_HASH")
	return gbm
}

// CreateGenInfoModule is used for testing, it commits to the [generic.GenInfoModule] columns,
func CreateGenInfoModule(
	comp *wizard.CompiledIOP,
	name string,
	size int,
) (gim generic.GenInfoModule) {
	createCol := common.CreateColFn(comp, name, size, pragmas.RightPadded)
	gim.HashHi = createCol("HASH_HI")
	gim.HashLo = createCol("HASH_LO")
	gim.IsHashHi = createCol("IS_HASH_HI")
	gim.IsHashLo = createCol("IS_HASH_LO")
	return gim
}

// it embeds  the expected hash (for the steam encoded inside gdm) inside gim columns.
func GenerateAndAssignGenInfoModule(
	run *wizard.ProverRuntime,
	gim *generic.GenInfoModule,
	gdm generic.GenDataModule,
	isHashHi, isHashLo []int,
) {

	var (
		hashHi      = common.NewVectorBuilder(gim.HashHi)
		hashLo      = common.NewVectorBuilder(gim.HashLo)
		isHashHiCol = common.NewVectorBuilder(gim.IsHashHi)
		isHashLoCol = common.NewVectorBuilder(gim.IsHashLo)
	)
	streams := gdm.ScanStreams(run)
	var res [][32]byte
	for _, stream := range streams {
		res = append(res, keccak.Hash(stream))

	}
	ctrHi := 0
	ctrLo := 0
	for i := range isHashHi {
		if isHashHi[i] == 1 {
			hashHi.PushHi(res[ctrHi])
			isHashHiCol.PushInt(1)
			ctrHi++
		} else {
			hashHi.PushInt(0)
			isHashHiCol.PushInt(0)
		}

		if isHashLo[i] == 1 {
			hashLo.PushLo(res[ctrLo])
			isHashLoCol.PushInt(1)
			ctrLo++
		} else {
			hashLo.PushInt(0)
			isHashLoCol.PushInt(0)
		}
	}

	hashHi.PadAndAssign(run)
	hashLo.PadAndAssign(run)
	isHashHiCol.PadAndAssign(run)
	isHashLoCol.PadAndAssign(run)
}
