package common

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitBytes(t *testing.T) {
	cases := []struct {
		input    []byte
		expected [][]byte
	}{
		{
			input:    []byte{0x11, 0x22, 0x33, 0x44},
			expected: [][]byte{{0x11, 0x22}, {0x33, 0x44}},
		},
		{
			input:    []byte{0x11, 0x22, 0x33},
			expected: [][]byte{{0x11, 0x22}, {0x33}},
		},
		{
			input:    []byte{0x11},
			expected: [][]byte{{0x11}},
		},
		{
			input:    []byte{},
			expected: [][]byte{},
		},
	}

	for _, cc := range cases {
		limbs := SplitBytes(cc.input)
		assert.Equal(t, cc.expected, limbs)
	}
}
