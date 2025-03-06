package test_utils

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/consensys/gnark/std/compress"
	"math/big"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
)

func printAsHexHint(_ *big.Int, ins, outs []*big.Int) error {
	byts := make([]byte, len(ins))
	for i := range byts {
		if b := ins[i].Uint64(); !ins[i].IsUint64() || b > 255 {
			return errors.New("not a byte")
		} else {
			byts[i] = byte(b)
		}
	}
	for i := range outs {
		outs[i].SetUint64(uint64(i))
	}
	fmt.Print("<PrintVarsAsHex> 0x")
	fmt.Println(StrBlocks(hex.EncodeToString(byts), 64))

	return nil
}

func PrintBytesAsHex(api frontend.API, v []frontend.Variable) {
	if _, err := api.Compiler().NewHint(printAsHexHint, 1, v...); err != nil {
		panic(err)
	}
}

func PrintBlocksAsHex(api frontend.API, blocks ...[32]frontend.Variable) {
	ins := make([]frontend.Variable, len(blocks)*32)
	k := 0
	for i := range blocks {
		for j := range blocks[i] {
			ins[k] = blocks[i][j]
			k++
		}
	}
	if _, err := api.Compiler().NewHint(printAsHexHint, 1, ins...); err != nil {
		panic(err)
	}
}

// PrintVarAsHex decomposes x into bytes and prints the whole thing
func PrintVarAsHex(api frontend.API, x frontend.Variable) {
	bits := api.ToBinary(x) // turn into bytes, inefficiently
	slices.Reverse(bits)
	n0 := len(bits) % 8
	bytes := make([]frontend.Variable, 1, (len(bits)+7)/8)
	two := big.NewInt(2)
	bytes[0] = compress.ReadNum(api, bits[:n0], two)
	bits = bits[n0:]
	for len(bits) != 0 {
		bytes = append(bytes, compress.ReadNum(api, bits[:8], two))
		bits = bits[8:]
	}
	PrintBytesAsHex(api, bytes)
}

func PrintBytesAsNum(b []byte) {
	var i big.Int
	i.SetBytes(b)
	fmt.Printf("%s 0x%s\n", i.Text(10), i.Text(16))
}

// StrBlocks injects a space in a string once every n characters
func StrBlocks(s string, n int) string {
	if len(s) < n {
		return s
	}
	var sb strings.Builder
	expectedLen := len(s) + (len(s)+n-1)/n - 1
	sb.Grow(expectedLen)

	sb.Write([]byte(s[:n]))
	for i := n; i < len(s); i += n {
		sb.WriteByte(' ')
		sb.Write([]byte(s[i:min(i+n, len(s))]))
	}

	if sb.Len() != expectedLen { // TODO delete
		panic(fmt.Errorf("expected length %d got %d", expectedLen, sb.Len()))
	}

	return sb.String()
}

// ProgressReporter prints percentages on console out; useful for monitoring long tasks
type ProgressReporter struct {
	n, pct int
}

func NewProgressReporter(n int) *ProgressReporter {
	return &ProgressReporter{
		n:   n,
		pct: -1,
	}
}

func (r *ProgressReporter) Update(i int) {
	if pct := i * 100 / r.n; pct != r.pct {
		r.pct = pct
		fmt.Printf("%d%%\n", pct)
	}
}

var snarkFunctionStore = make(map[uint64]func(frontend.API) []frontend.Variable) // todo make thread safe

type snarkFunctionTestCircuit struct {
	Outs   []frontend.Variable
	funcId uint64 // this workaround is necessary because deepEquals fails on objects with function fields
}

func (c *snarkFunctionTestCircuit) Define(api frontend.API) error {
	outs := snarkFunctionStore[c.funcId](api)
	delete(snarkFunctionStore, c.funcId)

	// todo replace with SliceEquals
	if len(outs) != len(c.Outs) {
		return errors.New("SnarkFunctionTest: unexpected number of output")
	}
	for i := range outs {
		api.AssertIsEqual(outs[i], c.Outs[i])
	}
	return nil
}

func SnarkFunctionTest(f func(frontend.API) []frontend.Variable, outs ...frontend.Variable) func(*testing.T) {

	return func(t *testing.T) {

		c := snarkFunctionTestCircuit{
			Outs: make([]frontend.Variable, len(outs)),
		}
		var b [8]byte
		_, err := rand.Read(b[:])
		assert.NoError(t, err)
		c.funcId = binary.BigEndian.Uint64(b[:])
		snarkFunctionStore[c.funcId] = f

		a := snarkFunctionTestCircuit{
			Outs: outs,
		}
		assert.NoError(t, test.IsSolved(&c, &a, ecc.BLS12_377.ScalarField()))
	}
}

func BlocksToHex(b ...[][32]byte) []string {
	res := make([]string, 0)
	for i := range b {
		for j := range b[i] {
			res = append(res, utils.HexEncodeToString(b[i][j][:]))
		}
	}
	return res
}
