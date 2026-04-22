// Package utils provides general-purpose utility functions used across the prover.
package utils

import (
	"encoding/hex"
	"strings"
)

// HexEncodeToString encodes a byte array into a hex string
func HexEncodeToString(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}

// HexDecodeString decodes a hex string into a byte array or error if it failed doing the conversion.
func HexDecodeString(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	return hex.DecodeString(s)
}
