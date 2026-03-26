package test_utils

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/assert"
)

// ProgressReporter prints percentages on console out; useful for monitoring long tasks
type ProgressReporter struct {
	n, pct int
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
