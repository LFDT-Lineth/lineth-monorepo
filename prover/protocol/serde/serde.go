// Standard Performance Run: go test ./... (Traces are completely gone; maximum speed ).
// Debug Run: go test -tags=trace ./... (Traces appear exactly as they did before ).

package serde

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unsafe"
)

var (
	serdeStructTag         = "serde"
	serdeStructTagOmit     = "omit"
	SerdeStructTagTestOmit = "test_omit"
)

// Serialize transforms a Go object into a binary-packed byte slice.
// The resulting slice is structured with a metadata header at the beginning,
// followed by a linearized heap of data.
func Serialize(v any) ([]byte, error) {
	enc := newEncoder()

	// Reserve space for the FileHeader at the start of the buffer (Offset 0).
	// We will come back and fill this in once we know the final PayloadOffset and DataSize.
	_ = enc.write(FileHeader{})

	// Start the recursive serialization of the object graph.
	rootOff, err := encode(enc, reflect.ValueOf(v))
	if err != nil {
		return nil, err
	}

	// Report all unregistered interface types discovered during encoding in one go,
	// so the caller can fix impl_registry.go without having to re-run repeatedly.
	if len(enc.missingTypes) > 0 {
		names := make([]string, 0, len(enc.missingTypes))
		for t := range enc.missingTypes {
			names = append(names, t.String())
		}
		sort.Strings(names)
		return nil, fmt.Errorf(
			"serialization failed: %d unregistered concrete type(s) need to be added to impl_registry.go:\n  - %s",
			len(names),
			strings.Join(names, "\n  - "),
		)
	}

	// Construct the final metadata. PayloadOff points to the "Root" object.
	finalHeader := FileHeader{
		Magic:       Magic,
		Version:     1,
		PayloadType: 0,
		PayloadOff:  int64(rootOff),
		DataSize:    enc.offset,
	}

	// Finalize the encoded buffer and "Patch" the reserved file header at Offset 0.
	// Using unsafe here allows us to overwrite the header in-place without re-allocating (Zero-Copy).
	// This essentially stores the final FileHeader at the start of our encoded buffer.
	b := enc.buf.Bytes()
	*(*FileHeader)(unsafe.Pointer(&b[0])) = finalHeader
	return b, nil
}

// Deserialize reconstructs a Go object from a byte slice (typically a memory-mapped file).
// IMPORTANT: This is a Zero-Copy operation. The reconstructed object (v) will contain
// pointers, slices, and strings that point directly into the input buffer 'b'.
// The caller MUST ensure that 'b' remains valid and unmodified for the lifetime of 'v'.
func Deserialize(b []byte, v any) error {

	if v == nil {
		return fmt.Errorf("nil pointer passed to Deserialize")
	}

	// Sanity check: ensure the buffer is at least as large as our header.
	if len(b) < int(SizeOf[FileHeader]()) {
		return fmt.Errorf("buffer too small to contain header")
	}

	// Map the header from the start of the buffer and verify the Magic identifier.
	header := (*FileHeader)(unsafe.Pointer(&b[0]))
	if header.Magic != Magic {
		return fmt.Errorf("invalid magic bytes: file is corrupted or not a valid archive")
	}

	// Handle the 'Null' payload case (e.g., if a nil pointer was serialized) by
	// setting the target to its zero-value (nil).
	if Ref(header.PayloadOff).IsNull() {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val.Elem().Set(reflect.Zero(val.Elem().Type()))
		}
		return nil
	}

	// Bound check: ensure the root object offset exists within the buffer.
	if Ref(header.PayloadOff) > Ref(len(b)) {
		return fmt.Errorf("root payload offset out of bounds")
	}

	// IMPORTANT: Ensure target 'v' is always a pointer so we can mutate it with the result.
	// This is hard-enforcement (found in any Go std. ser/de libs.) due to Go's pass-by-value semantics.
	// If otherwise, we wouldn't have a "hook" in the Go runtime to attach our reconstructed object graph to.
	// By requiring a pointer, we ensure the caller has provided a valid "landing pad" for the reconstructed data.
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("v must be a pointer to a struct or value")
	}

	// Init the decoder with the ptrMap to handle de-duplication of pointers
	// during the reconstruction process (preserving cycles).
	dec := &decoder{
		data:   b,
		ptrMap: make(map[int64]reflect.Value),
	}

	// Begin the "Pointer Swizzling" process starting from the PayloadOff.
	return dec.decode(val.Elem(), int64(header.PayloadOff))
}
