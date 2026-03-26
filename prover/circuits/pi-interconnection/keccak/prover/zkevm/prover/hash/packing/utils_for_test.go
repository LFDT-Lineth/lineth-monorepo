package packing

import (
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/sirupsen/logrus"
)

// It represents the importation struct for testing.
type dataTraceImported struct {
	isNewHash, nByte []int
	limb             [][]byte
}

// It generates data to fill up the importation columns.
func table(t *dataTraceImported, numHash, blockSize, size int) [][]byte {

	// choose the limbs for each hash
	// we set the limbs to less than maxNBytes  and then pad them to get maxNByte.
	var (
		limbs     = make([][][]byte, numHash)
		nByte     = make([][]int, numHash)
		isNewHash = make([][]int, numHash)
		rand      = rand.New(utils.NewRandSource(0)) // nolint
	)

	// build the stream for each hash.
	stream := make([][]byte, numHash)
	// number of bytes per hash
	s := make([]int, numHash)
	for i := 0; i < numHash; i++ {
		// assuming that a limb would take 15 bytes on average (pessimistic)
		// this is forced to prevent the number of blocks goes beyond maxNumBlocks.
		m := size / (numHash * 15)
		// number of limbs for the current hash
		// added +1 to prevent edge-cases
		nlimb := rand.IntN(m) + 1 //nolint

		s[i] = 0
		for j := 0; j < nlimb; j++ {
			// generate random bytes
			// choose a random length for the slice
			length := rand.IntN(MAXNBYTE) + 1 //nolint

			// generate random bytes
			slice := make([]byte, length)
			_, err := utils.ReadPseudoRand(rand, slice)
			if err != nil {
				logrus.Fatalf("error while generating random bytes: %s", err)
			}

			stream[i] = append(stream[i], slice...)
			// pad the limb to get maxNByte.
			r := toByte16(slice)
			limbs[i] = append(limbs[i], r[:])
			nByte[i] = append(nByte[i], len(slice))
			if j == 0 {
				isNewHash[i] = append(isNewHash[i], 1)
			} else {
				isNewHash[i] = append(isNewHash[i], 0)
			}

			s[i] += len(slice)
		}

	}

	// pad any required bytes to get to the blocksize for each hash.
	for k := 0; k < numHash; k++ {
		if s[k]%blockSize != 0 {
			n := (blockSize - s[k]%blockSize)
			for n > MAXNBYTE {
				// generate random bytes
				slice := make([]byte, MAXNBYTE)
				_, err := utils.ReadPseudoRand(rand, slice)
				if err != nil {
					logrus.Fatalf("error while generating random bytes: %s", err)
				}

				stream[k] = append(stream[k], slice...)
				r := toByte16(slice)
				limbs[k] = append(limbs[k], r[:])
				nByte[k] = append(nByte[k], len(slice))
				isNewHash[k] = append(isNewHash[k], 0)

				n = n - MAXNBYTE
				s[k] = s[k] + MAXNBYTE
			}
			// generate random bytes
			slice := make([]byte, n)
			_, err := utils.ReadPseudoRand(rand, slice)
			if err != nil {
				logrus.Fatalf("error while generating random bytes: %s", err)
			}
			s[k] = s[k] + n

			stream[k] = append(stream[k], slice...)
			r := toByte16(slice)
			limbs[k] = append(limbs[k], r[:])
			nByte[k] = append(nByte[k], len(slice))

			isNewHash[k] = append(isNewHash[k], 0)
		}

		if s[k]%blockSize != 0 {
			utils.Panic("Padding is not done correctly")
		}

	}

	// accumulate the tables from different hashes in a single table.
	var limbT [][]byte
	var nByteT, isNewHashT []int
	for k := 0; k < numHash; k++ {
		limbT = append(limbT, limbs[k]...)
		nByteT = append(nByteT, nByte[k]...)
		isNewHashT = append(isNewHashT, isNewHash[k]...)
	}

	t.limb = limbT
	t.nByte = nByteT
	t.isNewHash = isNewHashT

	// get the inputs for the hashes
	for i := range stream {
		if len(stream[i]) != s[i] {
			utils.Panic("stream is not set to the right length, stream length %v, what it  should be %v", len(stream[i]), s[i])
		}
		if len(stream[i])%blockSize != 0 {
			utils.Panic("padding was not correct")
		}
	}
	return stream
}

// It extends a short slice to [16]bytes.
func toByte16(b []byte) [16]byte {
	if len(b) > MAXNBYTE {
		utils.Panic("the length of input should not be greater than %v", MAXNBYTE)
	}
	n := MAXNBYTE - len(b)
	a := make([]byte, n)
	var c [MAXNBYTE]byte
	b = append(b, a...)
	copy(c[:], b)
	return c
}

const (
	TEST_IMPRTATION_COLUMN = "TEST_IMPORTATION_COLUMN"
)

// It creates the importation columns
func createImportationColumns(comp *wizard.CompiledIOP, size int) Importation {
	createCol := common.CreateColFn(comp, TEST_IMPRTATION_COLUMN, size, pragmas.RightPadded)
	res := Importation{
		IsNewHash: createCol("IsNewHash"),
		IsActive:  createCol("IsActive"),
		Limb:      createCol("Limb"),
		NByte:     createCol("Nbyte"),
	}
	return res
}

// it assigns the importation columns
func assignImportationColumns(run *wizard.ProverRuntime, imported *Importation, numHash, blockSize, targetSize int) {
	var t dataTraceImported
	_ = table(&t, numHash, blockSize, targetSize)

	u := make([]field.Element, len(t.limb))
	for i := range t.limb {
		u[i].SetBytes(t.limb[i][:])
	}
	a := smartvectors.ForTest(t.isNewHash...)
	aa := smartvectors.RightZeroPadded(smartvectors.IntoRegVec(a), targetSize)
	run.AssignColumn(imported.IsNewHash.GetColID(), aa)

	c := smartvectors.ForTest(t.nByte...)
	cc := smartvectors.RightZeroPadded(smartvectors.IntoRegVec(c), targetSize)
	run.AssignColumn(imported.NByte.GetColID(), cc)

	run.AssignColumn(imported.Limb.GetColID(), smartvectors.RightZeroPadded(u, targetSize))

	run.AssignColumn(imported.IsActive.GetColID(),
		smartvectors.RightZeroPadded(vector.Repeat(field.One(), len(t.limb)), targetSize))

}
