package types

import (
	"bytes"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

func TestFullBytes32(t *testing.T) {
	val := FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	written := "0x0000c5d200004601000086f70000233c0000927e00007db20000dcc7000003c00000e5000000b6530000ca820000273b00007bfa0000d80400005d850000a470"

	buf := &bytes.Buffer{}
	val.WriteTo(buf)
	written2 := utils.HexEncodeToString(buf.Bytes())
	assert.Equal(t, written, written2)

}
