package utils_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

func TestHexDecodeStringSuccess(t *testing.T) {
	tests := []struct {
		desc, input string
		want        []byte
	}{
		{"Empty string", "0x", []byte{}},
		{"Valid hex string with prefix", "0xabcdef", []byte{0xab, 0xcd, 0xef}},
		{"Valid hex string without prefix", "abcdef", []byte{0xab, 0xcd, 0xef}},
	}

	for i, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if got, err := utils.HexDecodeString(tt.input); err != nil {
				t.Errorf("test case %d: unexpected error: %s", i, err)
			} else if !bytes.Equal(got, tt.want) {
				t.Errorf("test case %d:\nwant: %q\ngot: %q", i, tt.want, got)
			}
		})
	}
}

func TestHexDecodeStringFailure(t *testing.T) {
	t.Run("InvalidHexString", func(t *testing.T) {
		if _, err := utils.HexDecodeString("not_a_hex_string"); err == nil {
			t.Errorf("expected error but got nil")
		} else {
			invByteErr := hex.InvalidByteError('n')
			if !errors.As(err, &invByteErr) {
				formatStr := "\nwant error: %s\ngot: %s"
				t.Errorf(formatStr, invByteErr, err)
			}
		}
	})

	t.Run("OddLenInput", func(t *testing.T) {
		if _, err := utils.HexDecodeString("abc"); err == nil {
			t.Errorf("expected error but got nil")
		} else if !errors.Is(err, hex.ErrLength) {
			formatStr := "\nwant error: %s\ngot: %s"
			t.Errorf(formatStr, hex.ErrLength, err)
		}
	})
}

func TestHexEncodeToString(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"Empty slice", []byte{}, "0x"},
		{"Single byte", []byte{0xab}, "0xab"},
		{"Multiple bytes", []byte{0xab, 0xcd, 0xef}, "0xabcdef"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.HexEncodeToString(tt.input)
			if got != tt.want {
				t.Errorf("test case %d: want %s, got %s", i, tt.want, got)
			}
		})
	}
}
