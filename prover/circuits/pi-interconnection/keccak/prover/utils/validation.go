package utils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Appends a wrapped error to `err` if x is not a valid hexstring. The err
// wrapper must be a string of the form "err message : %w". Size if the expected
// length of the string. Set to -1 to ignore the length. The length refers to
// the number of encoded bytes, not the number of characters.
func ValidateHexString(joinToErr *error, x, errWrapper string, size int) {

	// Be tolerant to hex strings without an hex prefix.
	x = strings.TrimPrefix(x, "0x")

	if _, err := hex.DecodeString(x); err != nil {
		*joinToErr = errors.Join(
			*joinToErr,
			fmt.Errorf(
				errWrapper,
				fmt.Errorf("could not decode hexstring `%v` : %w", x, err),
			),
		)
		return
	}

	if len(x)/2 != size && size > -1 {
		*joinToErr = errors.Join(
			*joinToErr,
			fmt.Errorf(
				errWrapper,
				fmt.Errorf(
					"`%v` encodes a %d bytes string, expected %v bytes",
					x, len(x)/2, size),
			),
		)
		return
	}
}

// Validate that the timestamps are increasing.
func ValidateTimestamps(joinToErr *error, timestamps ...uint) {

	for i := 1; i < len(timestamps); i++ {
		if timestamps[i] <= timestamps[i-1] {
			*joinToErr = errors.Join(
				*joinToErr,
				fmt.Errorf(
					"timestamps are not increasing : %v", timestamps,
				),
			)
		}
	}
}
