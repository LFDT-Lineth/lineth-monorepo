package sha2

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/uints"
	"github.com/consensys/gnark/std/permutation/sha2"
)

// SHA2Circuit is the gnark circuit (compiled as Plonk) used to check the Sha2
// compression function.
type SHA2Circuit struct {
	Instances []sha2BlockPermutationInstance `gnark:",public"`
}

func allocateSha2Circuit(nbInstances int) *SHA2Circuit {
	return &SHA2Circuit{
		Instances: make([]sha2BlockPermutationInstance, nbInstances),
	}
}

// Define implements the [frontend.Circuit] interface
func (sc *SHA2Circuit) Define(api frontend.API) error {
	for i := range sc.Instances {
		sc.Instances[i].checkSha2Permutation(api)
	}
	return nil
}

// sha2BlockPermutationInstance represents a instance of the sha2 block permutation.
type sha2BlockPermutationInstance struct {
	// prevDigest is the previous digest formatted as 8 uint32
	PrevDigest [2]frontend.Variable
	// the block formatted as [16]uint32
	Block [16]frontend.Variable
	// the current digest on 8 x uint32
	NewDigest [2]frontend.Variable
}

// checkSha2Permutation adds the constraints ensuring the correctness of the
// instance.
func (sbpi *sha2BlockPermutationInstance) checkSha2Permutation(api frontend.API) {

	uapi, err := uints.New[uints.U32](api)
	if err != nil {
		panic(fmt.Sprintf("unexpected error when instantiating `uapi`: %v", err.Error()))
	}

	var (
		// If the new digest is zero, then the block check is skipped as this is
		// considered a padding instance. The wizard should externally check that
		// NewDigest = 0x0 is forbidden.
		inpIsZero = api.Add(
			api.IsZero(sbpi.NewDigest[0]),
			api.IsZero(sbpi.NewDigest[1]),
		)
	)

	var (
		prevDigest = cast2xu128To8xU32s(api, sbpi.PrevDigest)
		newDigest  = cast2xu128To8xU32s(api, sbpi.NewDigest)
		blockBytes = [64]uints.U8{}
	)

	for i := range sbpi.Block {
		blockU32 := uapi.ValueOf(sbpi.Block[i])
		blockU8 := uapi.UnpackMSB(blockU32)
		copy(blockBytes[4*i:], blockU8)
	}

	recomputedNewDigest := sha2.Permute(uapi, prevDigest, blockBytes)

	for i := range recomputedNewDigest {
		// This checks that newDigest == recomputedDigest unless inpIsZero == 2
		api.AssertIsEqual(
			api.Mul(
				api.Sub(inpIsZero, 2),
				api.Sub(
					uapi.ToValue(recomputedNewDigest[i]),
					uapi.ToValue(newDigest[i]),
				),
			),
			0,
		)
	}
}

func cast2xu128To8xU32s(api frontend.API, v [2]frontend.Variable) [8]uints.U32 {

	var (
		u8Vars = append(
			toNBytes(api, v[0], 16),
			toNBytes(api, v[1], 16)...,
		)
		u8s     = make([]uints.U8, 32)
		u32s    = [8]uints.U32{}
		uapi, _ = uints.New[uints.U32](api)
	)

	for i := range u8Vars {
		// Converting this way instead of using the uapi constructor saves a
		// rangecheck.
		u8s[i] = uints.U8{Val: u8Vars[i]}
	}

	for i := range u32s {
		u32s[i] = uapi.PackMSB(u8s[4*i : 4*i+4]...)
	}

	return u32s
}
