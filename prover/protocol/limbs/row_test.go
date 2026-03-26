package limbs

import (
	"encoding/hex"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/assert"
)

func TestRowFromBytes(t *testing.T) {

	b, _e := hex.DecodeString("0000111122223333444455556666777788889999aaaabbbbccccddddeeeeffff")
	if _e != nil {
		panic(_e)
	}

	r := RowFromBytes[LittleEndian](b)
	if r.NumLimbs() != NbLimbU256 {
		t.Fatal("wrong number of limbs")
	}

	b2 := r.ToBytes()
	assert.Equalf(t, b, b2, "wrong bytes %x != %x", b, b2)

	b3 := r.ToBigEndianLimbs().ToBytes()
	assert.Equalf(t, b, b3, "wrong bytes %x != %x", b, b3)
}

func TestRowFromKoala(t *testing.T) {

	hi, lo := uint64(0x1200), uint64(0x5609)
	koala := field.NewElement(hi<<16 | lo)
	r := RowFromKoala[LittleEndian](koala, 256)
	rint := RowFromInt[LittleEndian](int(hi<<16|lo), 256)

	assert.Equal(t, r, rint)
	assert.Equal(t, 16, r.NumLimbs())
	assert.Equal(t, lo, r.T[0].Uint64())
	assert.Equal(t, hi, r.T[1].Uint64())
	assert.Equal(t, uint64(0), r.T[2].Uint64())
	assert.Equal(t, uint64(0), r.T[15].Uint64())

	r2 := RowFromKoala[BigEndian](koala, 256)
	assert.Equal(t, 16, r2.NumLimbs())
	assert.Equal(t, hi, r2.T[14].Uint64())
	assert.Equal(t, lo, r2.T[15].Uint64())
	assert.Equal(t, uint64(0), r2.T[0].Uint64())
	assert.Equal(t, uint64(0), r2.T[13].Uint64())

	bi := r.ToBigInt()
	assert.Equal(t, koala.Uint64(), bi.Uint64())

	bige := r.ToBigEndianLimbs().ToBigInt()
	assert.Equal(t, koala.Uint64(), bige.Uint64())

	ble, bge := r.ToBytes(), r2.ToBigEndianLimbs().ToBytes()
	assert.Equal(t, ble, bge)
}

func TestSplitOnBit(t *testing.T) {
	r := RowFromInt[LittleEndian](0x12345678, 32)
	rh, rl := r.SplitOnBit(16)
	assert.Equal(t, 1, rh.NumLimbs())
	assert.Equal(t, 1, rl.NumLimbs())
}
