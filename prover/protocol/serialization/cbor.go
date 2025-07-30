package serialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
)

var (
	cborEncMode cbor.EncMode
	cborDecMode cbor.DecMode
	encInitOnce sync.Once
	decInitOnce sync.Once
	bufferPool  = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	readerPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Reader)
		},
	}
)

func initCBOREncMode() {
	var err error
	cborEncMode, err = cbor.CoreDetEncOptions().EncMode()
	if err != nil {
		panic(fmt.Errorf("failed to create CBOR EncMode: %w", err))
	}
}

func initCBORDecMode() {
	var err error
	cborDecMode, err = cbor.DecOptions{
		MaxArrayElements: 134217728,
		MaxMapPairs:      134217728,
		MaxNestedLevels:  256,
	}.DecMode()
	if err != nil {
		panic(fmt.Errorf("failed to create CBOR DecMode: %w", err))
	}
}

func SerializeAnyWithCborPkg(x any) (cbor.RawMessage, error) {
	return encodeWithCBOR(x)
}

func DeserializeAnyWithCborPkg(data json.RawMessage, x any) error {
	return decodeWithCBOR(data, x)
}

// encodeWithCBOR serializes an interface{} object into CBOR using a pooled buffer.
// It will return an error on failure and is meant to be used on data and types that controlled
// by the current package.
func encodeWithCBOR(x any) (cbor.RawMessage, error) {
	encInitOnce.Do(initCBOREncMode)

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	enc := cborEncMode.NewEncoder(buf)
	if err := enc.Encode(x); err != nil {
		return nil, err
	}
	// Copy the bytes out before returning buffer to pool
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}

// decodeWithCBOR deserializes CBOR data into an object using a pooled reader.
func decodeWithCBOR(data json.RawMessage, x any) error {
	decInitOnce.Do(initCBORDecMode)

	r := readerPool.Get().(*bytes.Reader)
	r.Reset(data)
	defer readerPool.Put(r)

	dec := cborDecMode.NewDecoder(r)
	if err := dec.Decode(x); err != nil {
		return fmt.Errorf("cbor.Unmarshal failed: %w", err)
	}
	return nil
}
