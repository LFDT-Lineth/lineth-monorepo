package poseidon2

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
)

func TestHorizontalHash(t *testing.T) {

	tcases := []struct {
		Title     string
		InputHex  string
		OutputHex string
	}{
		{
			Title:    "address0",
			InputHex: "0x000035a50000e43d00003d31000095b400009cbf0000e78c0000d9440000115e0000000000000000000000000000000000000000000000000000aa2e000009db",
		},
		{
			Title:    "address",
			InputHex: "0x0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb",
		},
		{
			Title:    "bytes32",
			InputHex: "0x0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000aaaa0000bbbb0000eeee0000ffff0000eeee0000ffff0000eeee0000ffff0000eeee0000ffff",
		},
		{
			Title:    "account",
			InputHex: "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df70eb3b09c7b3948eb519f4dfe182d11d2790bdd2a3546f935709c507a5b56cee200006e490000e6670000820300007c0500005589000078700000e29f0000a5e5000052da0000f471000095520000131a00000abc0000e7790000daec00000a5d00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000053",
		},
	}

	for i := range tcases {

		t.Run(tcases[i].Title, func(t *testing.T) {

			input, hexErr := utils.HexDecodeString(tcases[i].InputHex)
			if hexErr != nil {
				panic(hexErr)
			}

			var (
				out      = poseidon2_koalabear.HashBytes(input)
				outHex   = utils.HexEncodeToString(out)
				numInput = utils.DivExact(len(input), 4)
				xs       []ifaces.Column
				hashing  *HashingCtx
				numRow   = 32
			)

			fmt.Printf("input = %x outHex = %v\n", input, outHex)

			define := func(b *wizard.Builder) {
				xs = make([]ifaces.Column, numInput)
				for i := 0; i < numInput; i++ {
					xs[i] = b.RegisterCommit(ifaces.ColID("X"+strconv.Itoa(i)), numRow)
				}
				hashing = HashOf(b.CompiledIOP, xs, "test")
			}

			assign := func(run *wizard.ProverRuntime) {
				for i := 0; i < numInput; i++ {
					var (
						inp          field.Element
						currInpBytes = input[field.Bytes*i : field.Bytes*(i+1)]
					)
					if err := inp.SetBytesCanonical(currInpBytes); err != nil {
						panic(err)
					}
					run.AssignColumn(xs[i].GetColID(), smartvectors.NewConstant(inp, numRow))
				}
				hashing.Run(run)
			}

			// This extracts the first row of the result as computed by the
			// the prover so that we can compare it to the value we computed
			// above.
			var (
				comp   = wizard.Compile(define, dummy.Compile)
				run    = wizard.RunProver(comp, assign, false)
				proof  = run.ExtractProof()
				result = hashing.Result()
				outs   = types.KoalaOctuplet{}
			)

			if err := wizard.Verify(comp, proof, false); err != nil {
				utils.Panic("error verifying proof: %v", err)
			}

			for i := range outs {
				outs[i] = result[i].GetColAssignmentAt(run, 0)
			}

			assert.Equal(t, outHex, outs.Hex())
		})
	}
}

func TestHorizontalHashRandom(t *testing.T) {

	var (
		maxNumCol = 100
		numRow    = 32
		rng       = rand.New(utils.NewRandSource(0)) // #nosec G404 -- test only
	)

	for i := range maxNumCol {
		t.Run(fmt.Sprintf("%v-columns", i+1), func(t *testing.T) {

			var (
				numCol  = i + 1
				xs      []ifaces.Column
				hashing = &HashingCtx{}
			)

			define := func(b *wizard.Builder) {
				xs = make([]ifaces.Column, numCol)
				for i := 0; i < numCol; i++ {
					xs[i] = b.RegisterCommit(ifaces.ColID("X"+strconv.Itoa(i)), numRow)
				}
				hashing = HashOf(b.CompiledIOP, xs, "test")
			}

			assign := func(run *wizard.ProverRuntime) {
				for i := 0; i < numCol; i++ {
					v := smartvectors.PseudoRand(rng, numRow)
					run.AssignColumn(xs[i].GetColID(), v)
				}
				hashing.Run(run)
			}

			var (
				comp  = wizard.Compile(define, dummy.Compile)
				run   = wizard.RunProver(comp, assign, false)
				proof = run.ExtractProof()
			)

			if err := wizard.Verify(comp, proof, false); err != nil {
				utils.Panic("error verifying proof: %v", err)
			}
		})
	}

}
