package types

import (
	"bytes"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/assert"
)

func TestFullBytes32(t *testing.T) {
	val := FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	written := "0x00000000000000000000000000000000e500b653ca82273b7bfad8045d85a47000000000000000000000000000000000c5d2460186f7233c927e7db2dcc703c0"
	buf := &bytes.Buffer{}
	val.WriteTo(buf)
	written2 := utils.HexEncodeToString(buf.Bytes())
	assert.Equal(t, written, written2)
}
