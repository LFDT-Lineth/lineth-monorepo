package proofserialization

// White-box, because the behaviour is unreachable from outside: driving the size
// guard through Encode would mean actually building a gigabyte-plus image.

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckImageSize(t *testing.T) {
	require.NoError(t, checkImageSize(0), "an empty image is within the region")
	require.NoError(t, checkImageSize(MaxImageSize), "exactly the region size still fits")

	err := checkImageSize(MaxImageSize + 1)
	require.ErrorContains(t, err, "exceeds the guest",
		"one byte over LENGTH(IN) cannot be loaded, so Encode must refuse it rather than "+
			"hand back an image the guest will truncate")
}
