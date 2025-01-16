package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func main() {

	var (
		limbs     = make([]field.Element, 32)
		nBytes    = make([]field.Element, 32)
		toHash    = make([]field.Element, 32)
		index     = make([]field.Element, 32)
		hashNum   = make([]field.Element, 32)
		rng       = rand.New(rand.NewChaCha8([32]byte{}))
		hashNums  = []int{0, 0, 0, 0, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0}
		toHashInt = []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
		oF        = files.MustOverwrite("./testdata/input.csv")
	)

	for i := range hashNum {

		if i == 0 {
			index[i] = field.Zero()
		} else if hashNums[i] != hashNums[i-1] {
			index[i] = field.Zero()
		} else if toHashInt[i] == 0 {
			index[i] = index[i-1]
		} else {
			index[i].Add(&index[i-1], new(field.Element).SetOne())
		}

		toHash[i] = field.NewElement(uint64(toHashInt[i]))
		hashNum[i] = field.NewElement(uint64(hashNums[i]))
		numBytesInt, numBytesF := randNBytes(rng)
		nBytes[i] = numBytesF
		limbs[i] = randLimbs(rng, numBytesInt)
	}

	fmt.Fprint(oF, "TO_HASH,HASH_NUM,INDEX,NBYTES,LIMBS\n")

	for i := range hashNums {
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

func randNBytes(rng *rand.Rand) (int, field.Element) {

	// nBytesInt must be in 1..=16
	var (
		nBytesInt = rng.Int31n(16) + 1
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
