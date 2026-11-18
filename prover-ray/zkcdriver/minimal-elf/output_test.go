package minimalelf

import "testing"

// TestOutputAndLogText_BuffersPastText checks that at the size cap, where .text
// is longest, both buffers start past its end.
func TestOutputAndLogText_BuffersPastText(t *testing.T) {
	full := make([]byte, outputAndLogMaxN)
	text := len(outputAndLogText(full, full))
	if text > outputBufferOffset {
		t.Fatalf(".text is %d bytes, past the output buffer at %d", text, outputBufferOffset)
	}
	if logBufferOffset < outputBufferOffset+outputAndLogMaxN {
		t.Fatalf("the log buffer at %d overlaps the output buffer at %d", logBufferOffset, outputBufferOffset)
	}
}
