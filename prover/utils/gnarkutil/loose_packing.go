package gnarkutil

import (
	"io"
)

// PackLoose packs the input bytes into blocks of elements,
// with each element's leftmost byte equal to 0.
func PackLoose(out io.Writer, input []byte, elemNbBytes, blockNbElems int) error {
	elemNbBytesUnpadded := elemNbBytes - 1
	nbElems := (len(input) + elemNbBytesUnpadded - 1) / elemNbBytesUnpadded
	nbBlocks := (nbElems + blockNbElems - 1) / blockNbElems
	zeroElement := make([]byte, elemNbBytes)
	for i := range nbElems {
		if _, err := out.Write(zeroElement[:1]); err != nil {
			return err
		}
		if _, err :=
			out.Write(input[i*elemNbBytesUnpadded : min(len(input), i*elemNbBytesUnpadded+elemNbBytesUnpadded)]); err != nil {
			return err
		}
	}

	// right pad the last element with bytes
	if _, err := out.Write(zeroElement[:nbElems*elemNbBytesUnpadded-len(input)]); err != nil {
		return err
	}

	// right pad the last block with elements
	for range nbBlocks*blockNbElems - nbElems {
		if _, err := out.Write(zeroElement); err != nil {
			return err
		}
	}
	return nil
}
