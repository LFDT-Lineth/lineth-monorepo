// Package types provides serialization and deserialization utilities for field elements and integers.
package types

import (
	"fmt"
	"strconv"

	"github.com/consensys/linea-monorepo/prover-ray/utils"
)

// DecodeQuotedHexString decodes a JSON-quoted hex string into raw bytes.
func DecodeQuotedHexString(b []byte) ([]byte, error) {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return nil, fmt.Errorf(
			"could not unmarshal hex string : expected a quoted string but got `%v`, error : %w",
			string(b), err,
		)
	}
	decoded, err := utils.HexDecodeString(unquoted)
	if err != nil {
		return nil, fmt.Errorf(
			"could not unmarshal hex string : expected an hex string but got `%v`, error : %w",
			unquoted, err,
		)
	}
	return decoded, nil
}

// MarshalHexBytesJSON marshals a byte slice as a quoted hex string for JSON.
func MarshalHexBytesJSON(b []byte) []byte {
	hexstring := utils.HexEncodeToString(b)
	return []byte(strconv.Quote(hexstring))
}
