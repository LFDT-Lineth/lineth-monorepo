package gkr

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

func TestAdd2(t *testing.T) {

	const (
		xID = 10001
		yID = 10002
		zID = 10003
	)

	var api API
	x := api.NewInput(xID)
	y := api.NewInput(yID)

	api.Export(api.Add(x, y), zID)

	c := api.Compile()
	var c2 Compiled
	require.NoError(t, c2.Deserialize(c.Serialize()))

	a := Assignment{
		xID: exts(2),
		yID: exts(4),
		zID: exts(6),
	}

	p := NewProverState(&c2, a)
	require.NotNil(t, p)

	var challenges []field.Ext
	for p.HasNext() {
		challenges = append(challenges, field.IntsToExt(int64(len(challenges)), 0, 0, 0, 0, 0))
		p.Next(challenges[len(challenges)-1])
	}

	aOuts := Assignment{
		zID: exts(6),
	}
	_, err := Verify(&c2, aOuts, p.Proof, challenges)
	require.NoError(t, err)
}

// exts lifts base-field values into the extension, one per instance.
func exts(v ...uint64) []field.Ext {
	res := make([]field.Ext, len(v))
	for i := range v {
		res[i] = field.Uint64ToExt(v[i])
	}
	return res
}
