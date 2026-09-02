package gkr

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

func TestAdd2(t *testing.T) {
	var api API
	x := api.NewInput("x")
	y := api.NewInput("y")
	api.Export(api.Add(x, y), "z")

	var api2 API
	api2.Deserialize(api.Serialize())

	p := NewProverState(&api2)
	require.NotNil(t, p)

	var challenges []field.Ext
	for p.HasNext() {
		challenges = append(challenges, field.IntsToExt(int64(len(challenges)), 0, 0, 0, 0, 0))
		p.Next(challenges[len(challenges)-1])
	}

	require.NoError(t, Verify(api2, p.Proof, challenges))
}
