package backend

import (
	"encoding/binary"
	"fmt"
)

// SSZ layout of the l2-execution public inputs, as the guest encodes them
// (riscv-guests/guest_common/src/core_serde/l2_execution_proof.zig): a fixed
// head with the fixed-size fields in wire order and a 4-byte offset in place
// of each of the three lists, then the lists.
const (
	publicInputsFixedSize = 284
	maxL2L1Messages       = 1 << 16
	maxFilteredAddresses  = 1 << 16
	maxMessagingBlocks    = 1 << 16
)

// decodeGuestPublicInputs decodes the guest_log frame, schema id || SSZ(public
// inputs), into the public inputs. Like the guest's own decoder, it accepts
// only the canonical encoding. The hash is checked by [checkGuestOutputFrame].
func decodeGuestPublicInputs(framed []byte) (PublicInputs, error) {
	if len(framed) < guestOutputSchemaIDSize {
		return PublicInputs{}, fmt.Errorf("%d bytes, shorter than the schema id", len(framed))
	}
	if id := binary.BigEndian.Uint16(framed); id != guestOutputSchemaID {
		return PublicInputs{}, fmt.Errorf("schema id is 0x%04x, want 0x%04x", id, guestOutputSchemaID)
	}
	body := framed[guestOutputSchemaIDSize:]
	if len(body) < publicInputsFixedSize {
		return PublicInputs{}, fmt.Errorf("public inputs are %d bytes, shorter than the %d-byte fixed head",
			len(body), publicInputsFixedSize)
	}

	var (
		pi PublicInputs
		r  = sszReader{buf: body}
	)
	pi.ParentBlockHash = r.hash()
	pi.EndBlockHash = r.hash()
	pi.EndBlockNumber = r.u64()
	pi.EndBlockTimestamp = r.u64()
	messagesStart := r.offset()
	pi.EndL1L2BridgeRollingHash = r.hash()
	pi.EndL1L2BridgeRollingHashMessageNumber = r.u64()
	pi.DynamicChainConfigHash = r.hash()
	pi.ParentFtxRollingHash = r.hash()
	pi.ParentFtxNumber = r.u64()
	pi.EndFtxRollingHash = r.hash()
	pi.EndProcessedFtxNumber = r.u64()
	filteredStart := r.offset()
	pi.TxFromsHash = r.hash()
	pi.BlockCount = r.u64()
	blocksStart := r.offset()

	// The first list starts right after the fixed head and each list ends
	// where the next one starts.
	if messagesStart != publicInputsFixedSize {
		return PublicInputs{}, fmt.Errorf("l2L1Messages offset is %d, want %d", messagesStart, publicInputsFixedSize)
	}
	if filteredStart < messagesStart || blocksStart < filteredStart || blocksStart > len(body) {
		return PublicInputs{}, fmt.Errorf("list offsets out of order or past the end: %d, %d, %d in %d bytes",
			messagesStart, filteredStart, blocksStart, len(body))
	}

	var err error
	if pi.L2L1Messages, err = decodeList(body[messagesStart:filteredStart], "l2L1Messages", 32, maxL2L1Messages,
		func(b []byte) [32]byte { return [32]byte(b) }); err != nil {
		return PublicInputs{}, err
	}
	if pi.FilteredAddresses, err = decodeList(body[filteredStart:blocksStart], "filteredAddresses", 20,
		maxFilteredAddresses, func(b []byte) [20]byte { return [20]byte(b) }); err != nil {
		return PublicInputs{}, err
	}
	if pi.L2MessagingBlocksOffsets, err = decodeList(body[blocksStart:], "l2MessagingBlocksOffsets", 2,
		maxMessagingBlocks, binary.LittleEndian.Uint16); err != nil {
		return PublicInputs{}, err
	}
	return pi, nil
}

// sszReader reads the fixed head of an SSZ container in order. The caller
// checks the length first.
type sszReader struct {
	buf []byte
	pos int
}

func (r *sszReader) hash() [32]byte {
	h := [32]byte(r.buf[r.pos:])
	r.pos += 32
	return h
}

func (r *sszReader) u64() uint64 {
	v := binary.LittleEndian.Uint64(r.buf[r.pos:])
	r.pos += 8
	return v
}

func (r *sszReader) offset() int {
	v := int(binary.LittleEndian.Uint32(r.buf[r.pos:]))
	r.pos += 4
	return v
}

// decodeList decodes an SSZ list of fixed-size elements of size bytes each,
// at most maxLen of them. An empty list is empty, not nil, as the native
// runner's output parses.
func decodeList[T any](region []byte, name string, size, maxLen int, decode func([]byte) T) ([]T, error) {
	if len(region)%size != 0 {
		return nil, fmt.Errorf("%s is %d bytes, not a multiple of %d", name, len(region), size)
	}
	n := len(region) / size
	if n > maxLen {
		return nil, fmt.Errorf("%s has %d elements, more than %d", name, n, maxLen)
	}
	out := make([]T, n)
	for i := range out {
		out[i] = decode(region[i*size : (i+1)*size])
	}
	return out, nil
}
