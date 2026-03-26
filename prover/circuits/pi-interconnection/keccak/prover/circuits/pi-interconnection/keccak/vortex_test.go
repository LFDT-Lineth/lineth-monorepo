package keccak

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	const maxNbKeccakF = 400
	var m *module
	t.Log("compiling")
	compiled := wizard.Compile(func(b *wizard.Builder) {
		m = NewCustomizedKeccak(b.CompiledIOP, maxNbKeccakF)
		var (
			laneSize   = m.keccak.Inputs.LaneInfo.Lanes.Size()
			hashHiSize = m.keccak.HashHi.Size()
		)
		b.InsertCommit(0, "ExpectedLane", laneSize)
		b.InsertCommit(0, "ExpectedIsActive", laneSize)
		b.InsertCommit(0, "ExpectedIsFirstLane", laneSize)
		b.InsertCommit(0, "ExpectedHashHi", hashHiSize)
		b.InsertCommit(0, "ExpectedHashLo", hashHiSize)

		assertEqual := func(u, v ifaces.ColID) {
			b.GlobalConstraint(ifaces.QueryID(u+"="+"v"), ifaces.ColumnAsVariable(b.Columns.GetHandle(u)).Sub(ifaces.ColumnAsVariable(b.Columns.GetHandle(v))))
		}

		assertEqual("Lane", "ExpectedLane")
		assertEqual("IsLaneActive", "ExpectedIsActive")
		assertEqual("IsFirstLaneOfNewHash", "ExpectedIsFirstLane")
		assertEqual("HASH_OUTPUT_Hash_Hi", "ExpectedHashHi")
		assertEqual("HASH_OUTPUT_Hash_Lo", "ExpectedHashLo")

	}, dummy.Compile)
	t.Log("proving")

	for i, c := range getTestCases(t) {
		if i != 3 {
			continue
		}
		proof := wizard.Prove(compiled, func(r *wizard.ProverRuntime) {
			m.AssignCustomizedKeccak(r, c.in)

			lanes, isLaneActive, isFirstLaneOfHash, hashHi, hashLo := AssignColumns(c.in, utils.NextPowerOfTwo(maxNbKeccakF*lanesPerBlock))

			// pad hashHiLo to the right length; TODO length is nextPowerOfTwo(maxNbKeccakF); move to assign?
			hashPaddingLen := m.keccak.HashHi.Size() - len(hashHi)
			hashHi = append(hashHi, make([]field.Element, hashPaddingLen)...)
			hashLo = append(hashLo, make([]field.Element, hashPaddingLen)...)

			r.AssignColumn("ExpectedLane", toVectorUint64(lanes))
			r.AssignColumn("ExpectedIsActive", smartvectors.ForTest(isLaneActive...)) // It's "for test" in the sense that it expects ints. No problem using it in a non-test context.
			r.AssignColumn("ExpectedIsFirstLane", smartvectors.ForTest(isFirstLaneOfHash...))
			r.AssignColumn("ExpectedHashHi", smartvectors.NewRegular(hashHi))
			r.AssignColumn("ExpectedHashLo", smartvectors.NewRegular(hashLo))
		})

		t.Log("verifying")
		assert.NoError(t, wizard.Verify(compiled, proof))
	}
}

func TestPureGoAssign(t *testing.T) {
	for i, c := range getTestCases(t) {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			assert.Equal(t, len(c.in), len(c.hash))
			lanes, active, isFirstLane, hashHi, hashLo := AssignColumns(c.in, len(c.lanes))
			for j := range c.hash {
				hi := hashHi[j].Bytes()
				lo := hashLo[j].Bytes()
				var buf [32]byte
				assert.True(t, bytes.Equal(hi[:16], buf[:16]))
				assert.True(t, bytes.Equal(lo[:16], buf[:16]))

				copy(buf[:], hi[16:])
				copy(buf[16:], lo[16:])

				assert.Equal(t, hex.EncodeToString(c.hash[j][:]), hex.EncodeToString(buf[:]))
			}

			for j := range c.lanes {
				var b [8]byte
				binary.BigEndian.PutUint64(b[:], lanes[j])
				assert.Equal(t, hex.EncodeToString(c.lanes[j][:]), hex.EncodeToString(b[:]))
				assert.Equal(t, c.isFirstLaneOfHash[j], isFirstLane[j])
				assert.Equal(t, c.isLaneActive[j], active[j])
			}
		})
	}
}

func toVectorUint64(in []uint64) smartvectors.SmartVector {
	e := make([]field.Element, len(in))
	for i := range in {
		e[i].SetUint64(in[i])
	}
	return smartvectors.NewRegular(e)
}

// AssignColumns to be used in the interconnection circuit assign function
func AssignColumns(in [][]byte, nbLanes int) (lanes []uint64, isLaneActive, isFirstLaneOfHash []int, hashHi, hashLo []field.Element) {
	hashHi = make([]field.Element, len(in))
	hashLo = make([]field.Element, len(in))
	lanes = make([]uint64, nbLanes)
	isFirstLaneOfHash = make([]int, nbLanes)
	isLaneActive = make([]int, nbLanes)

	laneI := 0
	for i := range in {
		isFirstLaneOfHash[laneI] = 1
		// pad and turn into lanes
		nbBlocks := 1 + len(in[i])/136
		for j := 0; j < nbBlocks; j++ {
			var block [136]byte
			copy(block[:], in[i][j*136:])
			if j == nbBlocks-1 {
				block[len(in[i])-j*136] = 1 // dst
				block[135] |= 0x80          // end marker
			}
			for k := 0; k < 17; k++ {
				isLaneActive[laneI] = 1
				lanes[laneI] = binary.BigEndian.Uint64(block[k*8 : k*8+8])
				laneI++
			}
		}
		hash := utils.KeccakHash(in[i])
		hashHi[i].SetBytes(hash[:16])
		hashLo[i].SetBytes(hash[16:])

	}

	return
}
