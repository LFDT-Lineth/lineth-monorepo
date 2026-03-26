package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKoalaOctupletFromHex(t *testing.T) {

	inputs := []string{
		"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6",
		"0x4e823a281ec61ac1032fcc5437fc33fd0f126226288e1c806fa557f101808af4",
	}

	for i := range inputs {
		ko := MustHexToKoalabearOctuplet(inputs[i])
		back := ko.Hex()
		assert.Equal(t, inputs[i], back)
	}
}

func TestKoalaOctupletFromJson(t *testing.T) {

	inputs := []string{
		"\"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6\"",
	}

	for i := range inputs {

		x := KoalaOctuplet{}
		if err := x.UnmarshalJSON([]byte(inputs[i])); err != nil {
			t.Fatal(err)
		}

		back, err := json.Marshal(x)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, inputs[i], string(back))
	}
}
