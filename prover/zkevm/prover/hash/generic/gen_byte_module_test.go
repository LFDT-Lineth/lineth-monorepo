package generic

import (
	"bytes"
	"io"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name        string
	streams     [][]byte
	bytesPerRow int
}

func TestScanByteStream(t *testing.T) {
	testCases := []testCase{
		{
			name: "2 bytes per row",
			streams: [][]byte{
				mustDecodeHex("0x00112233445566778899aabbccddeeff"),
			},
			bytesPerRow: 2,
		},
		{
			name: "1 limb columns per row",
			streams: [][]byte{
				mustDecodeHex("0x00112233445566778899aabbccddeeff"),
			},
			bytesPerRow: 1,
		},
		{
			name: "arbitrary length, 5 bytes per row",
			streams: [][]byte{
				mustDecodeHex("0xaaafffffff789797979fff12332345680xaaafffffff789797979fff"),
			},
			bytesPerRow: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testScanByteStream)
	}
}

func (tc *testCase) testScanByteStream(t *testing.T) {

	if tc.bytesPerRow == 0 {
		utils.Panic("bytesPerRow must be > 0")
	}

	var (
		gdm              GenDataModule
		recoveredStreams [][]byte
	)

	comp := wizard.Compile(func(build *wizard.Builder) {
		gdm = GenDataModule{
			HashNum: build.RegisterCommit("A", 32),
			Limbs:   limbs.NewUint128Be(build.CompiledIOP, "B", 32),
			ToHash:  build.RegisterCommit("C", 32),
			NBytes:  build.RegisterCommit("D", 32),
			Index:   build.RegisterCommit("E", 32),
		}
	})

	_ = wizard.Prove(comp, func(run *wizard.ProverRuntime) {

		tc.assignGdbFromStream(run, &gdm)
		recoveredStreams = gdm.ScanStreams(run)
	})

	for i := range tc.streams {
		assert.Equalf(t,
			utils.HexEncodeToString(tc.streams[i]),
			utils.HexEncodeToString(recoveredStreams[i]),
			"position %v", i,
		)
	}
}

func (tc *testCase) assignGdbFromStream(run *wizard.ProverRuntime, gdm *GenDataModule) {

	var (
		hashNum = common.NewVectorBuilder(gdm.HashNum)
		nbBytes = common.NewVectorBuilder(gdm.NBytes)
		toHash  = common.NewVectorBuilder(gdm.ToHash)
		index   = common.NewVectorBuilder(gdm.Index)
		limbs   = limbs.NewVectorBuilder(gdm.Limbs.AsDynSize())
	)

	// Iterate over the streams in test case
	for hashID, currStream := range tc.streams {

		var (
			ctr        = 0
			dataReader = bytes.NewReader(currStream)
		)

	currStreamLoop:
		for {

			buf := make([]byte, tc.bytesPerRow)
			n, err := dataReader.Read(buf)

			switch {
			case err == nil && n > 0:
				buf := append(buf, make([]byte, 16-len(buf))...)
				limbs.PushBytes(buf)
				hashNum.PushInt(hashID + 1)
				index.PushInt(ctr)
				nbBytes.PushInt(n)
				toHash.PushOne()

			case err == io.EOF || n == 0:
				// This goes to the next stream
				break currStreamLoop

			default:
				utils.Panic("unexpected error: %v", err)
			}

			ctr++
		}
	}

	limbs.PadAndAssignZero(run)
	hashNum.PadAndAssign(run, field.Zero())
	nbBytes.PadAndAssign(run, field.Zero())
	toHash.PadAndAssign(run, field.Zero())
	index.PadAndAssign(run)
}

func mustDecodeHex(s string) []byte {
	b, _ := utils.HexDecodeString(s)
	return b
}
