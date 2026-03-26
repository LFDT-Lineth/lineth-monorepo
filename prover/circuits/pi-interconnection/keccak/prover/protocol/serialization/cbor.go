package serialization

import (
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
)

var (
	cborEncMode cbor.EncMode
	cborDecMode cbor.DecMode
	encInitOnce sync.Once
	decInitOnce sync.Once
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

// encodeWithCBOR: single-step Marshal, avoids buffer growth + extra copy
func encodeWithCBOR(x any) (cbor.RawMessage, error) {
	encInitOnce.Do(initCBOREncMode)
	return cborEncMode.Marshal(x)
}

// decodeWithCBOR: single-step Unmarshal (no reader pool)
func decodeWithCBOR(data []byte, x any) error {
	decInitOnce.Do(initCBORDecMode)
	return cborDecMode.Unmarshal(data, x)
}
