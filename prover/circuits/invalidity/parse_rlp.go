package invalidity

import (
	"errors"

	"github.com/consensys/gnark/frontend"
)

// ExtractNonceFromRLP extracts the nonce from RLP-encoded Dynamic Fee (EIP-1559) transaction bytes.
//
// This function only supports EIP-1559 transactions. The transaction structure is:
// - Byte 0: transaction type (0x02)
// - Followed by RLP list containing: [chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList]
//
// The nonce is the second field (index 1) in the RLP list.
//
// Parameters:
// - txBytes: A byte slice containing the RLP-encoded EIP-1559 transaction.
//
// Returns:
// - uint64: The nonce value extracted from the transaction.
// - error: An error if the input bytes are invalid, not an EIP-1559 transaction, or the RLP encoding is malformed.
//
// RLP Encoding Rules for the nonce:
//   - If the first byte is less than 0x80, the nonce is encoded as a single byte.
//   - If the first byte is between 0x80 and 0xb7, the nonce is length-prefixed (short encoding).
//   - If the first byte is between 0xb8 and 0xbf, the nonce is length-prefixed (long encoding).

func ExtractNonceFromRLP(txBytes []byte) (uint64, error) {
	if len(txBytes) == 0 {
		return 0, errors.New("empty transaction bytes")
	}

	// Verify this is a Dynamic Fee transaction (EIP-1559)
	txType := txBytes[0]
	if txType != 0x02 {
		return 0, errors.New("unsupported transaction type: only EIP-1559 (type 0x02) is supported")
	}

	// Skip the transaction type byte
	txBytes = txBytes[1:]

	// Parse the RLP list
	offset, err := parseRLPList(txBytes)
	if err != nil {
		return 0, err
	}

	// Extract the nonce (second field, index 1, after chainId)
	nonceBytes, err := extractRLPField(txBytes, offset, 1)
	if err != nil {
		return 0, err
	}

	// Convert the byte array to uint64
	return bytesToUint64(nonceBytes), nil
}

// ExtractNonceFromRLPZk extracts the nonce from an RLP-encoded Dynamic Fee (EIP-1559) transaction
// in a ZK circuit context.
//
// This function only supports EIP-1559 transactions. The transaction structure is:
// - Byte 0: tx type (0x02)
// - Byte 1+: RLP list (short list 0xc0-0xf7, or long list 0xf8-0xff with length bytes)
// - After list prefix: chainId (field index 0, variable length)
// - After chainId: nonce (field index 1, variable length)
//
// The function handles variable-length chainId and nonce fields according to RLP encoding rules:
// - Single byte value (< 0x80): The byte itself is the value
// - Short length-prefixed (0x80-0xb7): Length = firstByte - 0x80, followed by value bytes
func ExtractNonceFromRLPZk(api frontend.API, rawTx []frontend.Variable) frontend.Variable {
	// For Dynamic Fee transactions (EIP-1559):
	// - Byte 0: tx type (0x02)
	// - Byte 1+: RLP list prefix (variable length for long lists)
	// - After list prefix: chainId (field index 0, variable length)
	// - After chainId: nonce (field index 1, variable length)

	// Calculate the offset after the RLP list prefix
	// Short list (0xc0-0xf7): offset = 2 (1 byte tx type + 1 byte prefix)
	// Long list (0xf8-0xff): offset = 2 + (prefix - 0xf7) length bytes
	listPrefixOffset := getRLPListDataOffsetZk(api, rawTx)

	// Get the chainId byte at the calculated offset
	chainIdByte := getValueAtOffset(api, rawTx, listPrefixOffset)

	// Determine the length of chainId field:
	// - If chainIdByte < 0x80: single byte value, length = 1
	// - If chainIdByte == 0x80: empty value (0), length = 1
	// - If 0x80 < chainIdByte <= 0xb7: short length-prefixed, length = 1 + (chainIdByte - 0x80)

	// Check if chainId is a single byte value (<= 0x80)
	// In these cases, the chainId takes exactly 1 byte
	chainIdIsSingleByte := api.Sub(frontend.Variable(1), isGreaterThan(api, chainIdByte, frontend.Variable(0x80)))

	// Calculate the offset to the nonce field
	// If chainId is single byte (<= 0x80): offset = listPrefixOffset + 1
	// If chainId is short length-prefixed (0x81 to 0xb7): offset = listPrefixOffset + 1 + (chainIdByte - 0x80)
	chainIdLen := api.Select(chainIdIsSingleByte,
		frontend.Variable(1),
		api.Add(frontend.Variable(1), api.Sub(chainIdByte, frontend.Variable(0x80))))
	nonceOffset := api.Add(listPrefixOffset, chainIdLen)

	// Get the first byte of the nonce field using dynamic indexing
	nonceByte := getValueAtOffset(api, rawTx, nonceOffset)

	// Check if nonce is a single-byte value (< 0x80)
	nonceIsSingleByte := api.Sub(frontend.Variable(1), isGreaterOrEqual(api, nonceByte, frontend.Variable(0x80)))

	// For single-byte nonce, the value is the byte itself
	singleByteNonce := nonceByte

	// For length-prefixed nonce (0x80 <= nonceByte <= 0xb7):
	// The length is nonceByte - 0x80, and the actual nonce bytes follow
	nonceLen := api.Sub(nonceByte, frontend.Variable(0x80))

	// Reconstruct the nonce value from the following bytes
	// We support nonce up to 8 bytes (uint64)
	shortNonce := frontend.Variable(0)
	for i := 1; i <= 8; i++ {
		// Check if this index is within the nonce length
		isWithinLen := isLessThan(api, frontend.Variable(i-1), nonceLen)

		// Get the byte at offset + i
		byteVal := getValueAtOffset(api, rawTx, api.Add(nonceOffset, frontend.Variable(i)))

		// Only shift and add if we're still within the nonce bytes
		// If not within length, keep the current value unchanged
		newNonce := api.Add(api.Mul(shortNonce, frontend.Variable(256)), byteVal)
		shortNonce = api.Select(isWithinLen, newNonce, shortNonce)
	}

	// Select the final nonce value based on whether it's single-byte or length-prefixed
	nonce := api.Select(nonceIsSingleByte, singleByteNonce, shortNonce)

	return nonce
}

// ExtractTxCostFromRLP extracts the transaction cost from RLP-encoded Dynamic Fee (EIP-1559) transaction bytes.
//
// Transaction cost = value + gasLimit × maxFeePerGas
//
// This is used for the "invalid balance" check: if cost > sender.Balance, the transaction is invalid.
//
// This function only supports EIP-1559 transactions. The transaction structure is:
// - Byte 0: transaction type (0x02)
// - Followed by RLP list containing: [chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList]
//
// Required fields:
// - maxFeePerGas: field index 3
// - gasLimit: field index 4
// - value: field index 6
//
// Parameters:
// - txBytes: A byte slice containing the RLP-encoded EIP-1559 transaction.
//
// Returns:
// - uint64: The transaction cost (value + gasLimit * maxFeePerGas).
// - error: An error if the input bytes are invalid, not an EIP-1559 transaction, or the RLP encoding is malformed.
func ExtractTxCostFromRLP(txBytes []byte) (uint64, error) {
	if len(txBytes) == 0 {
		return 0, errors.New("empty transaction bytes")
	}

	// Verify this is a Dynamic Fee transaction (EIP-1559)
	txType := txBytes[0]
	if txType != 0x02 {
		return 0, errors.New("unsupported transaction type: only EIP-1559 (type 0x02) is supported")
	}

	// Skip the transaction type byte
	txBytes = txBytes[1:]

	// Parse the RLP list
	offset, err := parseRLPList(txBytes)
	if err != nil {
		return 0, err
	}

	// Extract maxFeePerGas (field index 3)
	maxFeePerGasBytes, err := extractRLPField(txBytes, offset, 3)
	if err != nil {
		return 0, err
	}
	maxFeePerGas := bytesToUint64(maxFeePerGasBytes)

	// Extract gasLimit (field index 4)
	gasLimitBytes, err := extractRLPField(txBytes, offset, 4)
	if err != nil {
		return 0, err
	}
	gasLimit := bytesToUint64(gasLimitBytes)

	// Extract value (field index 6)
	valueBytes, err := extractRLPField(txBytes, offset, 6)
	if err != nil {
		return 0, err
	}
	value := bytesToUint64(valueBytes)

	// Calculate transaction cost: value + gasLimit * maxFeePerGas
	cost := value + gasLimit*maxFeePerGas

	return cost, nil
}

// ExtractTxCostFromRLPZk extracts the transaction cost from an RLP-encoded Dynamic Fee (EIP-1559) transaction
// in a ZK circuit context.
//
// Transaction cost = value + gasLimit × maxFeePerGas
//
// This is used for the "invalid balance" check: if cost > sender.Balance, the transaction is invalid.
//
// This function only supports EIP-1559 transactions. The transaction structure is:
// - Byte 0: transaction type (0x02)
// - Byte 1+: RLP list (short list 0xc0-0xf7, or long list 0xf8-0xff with length bytes)
// - Fields: [chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList]
//
// Required fields:
// - maxFeePerGas: field index 3
// - gasLimit: field index 4
// - value: field index 6
//
// The function handles variable-length fields according to RLP encoding rules.
//
// Note: This implementation supports values up to 32 bytes (256 bits) for full Ethereum compatibility.
// The result is stored in a field element which can hold values up to ~253 bits on BLS12-377.
func ExtractTxCostFromRLPZk(api frontend.API, rawTx []frontend.Variable) frontend.Variable {
	// For Dynamic Fee transactions (EIP-1559):
	// - Byte 0: tx type (0x02)
	// - Byte 1+: RLP list prefix (variable length for long lists)
	// - Fields: [chainId(0), nonce(1), maxPriorityFeePerGas(2), maxFeePerGas(3), gasLimit(4), to(5), value(6), ...]

	// Calculate the offset after the RLP list prefix
	offset := getRLPListDataOffsetZk(api, rawTx)

	// Skip fields 0-2: chainId, nonce, maxPriorityFeePerGas
	for fieldIdx := 0; fieldIdx < 3; fieldIdx++ {
		offset = skipRLPFieldZk(api, rawTx, offset)
	}

	// Extract maxFeePerGas (field index 3)
	maxFeePerGas := extractRLPFieldValueZk(api, rawTx, offset)
	offset = skipRLPFieldZk(api, rawTx, offset)

	// Extract gasLimit (field index 4)
	gasLimit := extractRLPFieldValueZk(api, rawTx, offset)
	offset = skipRLPFieldZk(api, rawTx, offset)

	// Skip field 5: to address
	offset = skipRLPFieldZk(api, rawTx, offset)

	// Extract value (field index 6)
	value := extractRLPFieldValueZk(api, rawTx, offset)

	// Calculate transaction cost: value + gasLimit * maxFeePerGas
	cost := api.Add(value, api.Mul(gasLimit, maxFeePerGas))

	return cost
}

// extractRLPFieldValueZk extracts the numeric value of an RLP field at the given offset.
// Handles single-byte values (< 0x80) and short length-prefixed values (0x80-0xb7).
// Returns the value as a frontend.Variable.
//
// Supports up to 32 bytes (256 bits) for full Ethereum value compatibility.
// The result fits in BLS12-377 scalar field (~253 bits), which is sufficient for
// all practical Ethereum values (max supply ~120M ETH = ~120e24 wei < 2^87).
func extractRLPFieldValueZk(api frontend.API, rawTx []frontend.Variable, offset frontend.Variable) frontend.Variable {
	// Get the first byte of the field
	firstByte := getValueAtOffset(api, rawTx, offset)

	// Check if it's a single-byte value (< 0x80)
	isSingleByte := api.Sub(frontend.Variable(1), isGreaterOrEqual(api, firstByte, frontend.Variable(0x80)))

	// For single-byte value, the value is the byte itself
	singleByteValue := firstByte

	// For length-prefixed value (0x80 <= firstByte <= 0xb7):
	// The length is firstByte - 0x80, and the actual value bytes follow
	fieldLen := api.Sub(firstByte, frontend.Variable(0x80))

	// Reconstruct the value from the following bytes (up to 32 bytes for 256-bit values)
	multiByteValue := frontend.Variable(0)
	for i := 1; i <= 32; i++ {
		// Check if this index is within the field length
		isWithinLen := isLessThan(api, frontend.Variable(i-1), fieldLen)

		// Get the byte at offset + i
		byteVal := getValueAtOffset(api, rawTx, api.Add(offset, frontend.Variable(i)))

		// Only shift and add if we're still within the field bytes
		newValue := api.Add(api.Mul(multiByteValue, frontend.Variable(256)), byteVal)
		multiByteValue = api.Select(isWithinLen, newValue, multiByteValue)
	}

	// Select the final value based on whether it's single-byte or length-prefixed
	return api.Select(isSingleByte, singleByteValue, multiByteValue)
}

// getRLPListDataOffsetZk calculates the offset to the first field in an RLP list
// for EIP-1559 transactions in a ZK circuit context.
//
// EIP-1559 transaction structure:
// - Byte 0: tx type (0x02)
// - Byte 1: RLP list prefix
//   - Short list (0xc0-0xf7): data starts at byte 2
//   - Long list (0xf8-0xff): (prefix - 0xf7) length bytes follow, then data
//
// Returns the offset to the first field (chainId) in the transaction.
func getRLPListDataOffsetZk(api frontend.API, rawTx []frontend.Variable) frontend.Variable {
	// Get the RLP list prefix byte (at index 1, after tx type)
	listPrefix := rawTx[1]

	// Check if it's a short list (0xc0 <= prefix <= 0xf7)
	// In this case, data starts immediately after the prefix at offset 2
	isShortList := api.Sub(frontend.Variable(1), isGreaterThan(api, listPrefix, frontend.Variable(0xf7)))

	// For long list (0xf8 <= prefix <= 0xff):
	// The number of length bytes is (prefix - 0xf7)
	// Data starts at offset 2 + (prefix - 0xf7)
	lengthOfLength := api.Sub(listPrefix, frontend.Variable(0xf7))
	longListOffset := api.Add(frontend.Variable(2), lengthOfLength)

	// Select the correct offset based on list type
	// Short list: offset = 2
	// Long list: offset = 2 + (prefix - 0xf7)
	return api.Select(isShortList, frontend.Variable(2), longListOffset)
}

// skipRLPFieldZk calculates the offset after skipping one RLP field in a ZK circuit context.
// Supports single-byte values (< 0x80) and short length-prefixed values (0x80-0xb7, up to 55 bytes).
// This is sufficient for EIP-1559 fields: chainId, nonce, maxPriorityFeePerGas, maxFeePerGas,
// gasLimit (all ≤ 8 bytes), and to address (20 bytes).
func skipRLPFieldZk(api frontend.API, rawTx []frontend.Variable, offset frontend.Variable) frontend.Variable {
	// Get the first byte of the field
	firstByte := getValueAtOffset(api, rawTx, offset)

	// Check field type:
	// - Single byte (< 0x80): length = 1
	// - Empty or short string (0x80 <= byte <= 0xb7): length = 1 + (byte - 0x80)
	// - Long string (0xb8 <= byte <= 0xbf): length = 1 + (byte - 0xb7) + actual_length (complex, not fully supported)

	// Check if it's a single byte value (< 0x80)
	isSingleByte := api.Sub(frontend.Variable(1), isGreaterOrEqual(api, firstByte, frontend.Variable(0x80)))

	// Check if it's a short length-prefixed value (0x80 <= byte <= 0xb7)
	isShortString := api.And(
		isGreaterOrEqual(api, firstByte, frontend.Variable(0x80)),
		api.Sub(frontend.Variable(1), isGreaterThan(api, firstByte, frontend.Variable(0xb7))),
	)

	// Calculate field length for each case
	singleByteLen := frontend.Variable(1)
	shortStringLen := api.Add(frontend.Variable(1), api.Sub(firstByte, frontend.Variable(0x80)))

	// Select the correct length
	fieldLen := api.Select(isSingleByte, singleByteLen, api.Select(isShortString, shortStringLen, frontend.Variable(1)))

	// Return the new offset
	return api.Add(offset, fieldLen)
}

// isGreaterThan returns 1 if a > b, 0 otherwise
// Uses api.Cmp which returns -1 if a < b, 0 if a == b, 1 if a > b
func isGreaterThan(api frontend.API, a, b frontend.Variable) frontend.Variable {
	cmp := api.Cmp(a, b)
	// cmp is 1 if a > b, so we check if cmp == 1
	return api.IsZero(api.Sub(cmp, frontend.Variable(1)))
}

// isGreaterOrEqual returns 1 if a >= b, 0 otherwise
func isGreaterOrEqual(api frontend.API, a, b frontend.Variable) frontend.Variable {
	cmp := api.Cmp(a, b)
	// cmp is 0 or 1 if a >= b, so we check if cmp != -1
	// In field arithmetic, -1 is represented as p-1 (a large number)
	// Instead, check if a == b OR a > b
	isEqual := api.IsZero(api.Sub(a, b))
	isGreater := api.IsZero(api.Sub(cmp, frontend.Variable(1)))
	return api.Or(isEqual, isGreater)
}

// isLessThan returns 1 if a < b, 0 otherwise
func isLessThan(api frontend.API, a, b frontend.Variable) frontend.Variable {
	return isGreaterThan(api, b, a)
}

// parseRLPList parses the RLP list and calculates the offset of the first field.
// - The function correctly identifies whether the list is short or long based on the first byte.
// - It calculates the offset of the first field based on the length of the
// list.
//
// RLP lists are encoded as follows:
// - Short lists (length <= 55 bytes): The first byte is between 0xc0 and 0xf7.
//   - The length of the list is encoded in the first byte (0xc0 + length).
//   - The offset of the first field is 1 (after the length byte).
//
// - Long lists (length > 55 bytes): The first byte is between 0xf8 and 0xff.
//   - The length of the length field is encoded in the first byte (0xf7 + length of length).
//   - The actual length of the list is encoded in the next bytes.
//   - The offset of the first field is after the length bytes.

func parseRLPList(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, errors.New("empty RLP data")
	}

	firstByte := data[0]
	if firstByte < 0xc0 {
		return 0, errors.New("not an RLP list")
	}

	if firstByte <= 0xf7 {
		// Short list
		return 1, nil // Offset starts after the length byte
	} else {
		// Long list
		lengthOfLength := int(firstByte - 0xf7)
		if len(data) < 1+lengthOfLength {
			return 0, errors.New("invalid RLP encoding: insufficient bytes for list length")
		}
		return 1 + lengthOfLength, nil // Offset starts after the length bytes
	}
}

// extractRLPField extracts the nth field from an RLP list.
// - This function handles all possible RLP field types (single-byte, short, and long).
// - It iterates through the fields in the list and stops when the specified
// field index is reached.
//
// RLP fields are encoded as follows:
// - Single-byte values (0x00 to 0x7f): The value is encoded directly in the first byte.
// - Short length-prefixed values (0x80 to 0xb7): The first byte indicates the length of the value (0x80 + length).
//   - The actual value follows the length byte.
//
// - Long length-prefixed values (0xb8 to 0xbf): The first byte indicates the length of the length field (0xb7 + length of length).
//   - The actual length of the value is encoded in the next bytes.
//   - The value follows the length bytes.
//
// Steps:
// 1. Iterate through the fields in the RLP list until the specified field index is reached.
// 2. For each field, determine its type (single-byte, short length-prefixed, or long length-prefixed).
// 3. Extract the field and return it as a byte slice.

func extractRLPField(data []byte, offset int, fieldIndex int) ([]byte, error) {
	for i := 0; i <= fieldIndex; i++ {
		if offset >= len(data) {
			return nil, errors.New("invalid RLP encoding: insufficient bytes for field")
		}

		firstByte := data[offset]
		if firstByte < 0x80 {
			// Single-byte value
			if i == fieldIndex {
				return data[offset : offset+1], nil
			}
			offset++
		} else if firstByte <= 0xb7 {
			// Short length-prefixed value
			length := int(firstByte - 0x80)
			if offset+1+length > len(data) {
				return nil, errors.New("invalid RLP encoding: insufficient bytes for field")
			}
			if i == fieldIndex {
				return data[offset+1 : offset+1+length], nil
			}
			offset += 1 + length
		} else if firstByte <= 0xbf {
			// Long length-prefixed value
			lengthOfLength := int(firstByte - 0xb7)
			if offset+1+lengthOfLength > len(data) {
				return nil, errors.New("invalid RLP encoding: insufficient bytes for field length")
			}
			length := int(bytesToUint64(data[offset+1 : offset+1+lengthOfLength]))
			if offset+1+lengthOfLength+length > len(data) {
				return nil, errors.New("invalid RLP encoding: insufficient bytes for field")
			}
			if i == fieldIndex {
				return data[offset+1+lengthOfLength : offset+1+lengthOfLength+length], nil
			}
			offset += 1 + lengthOfLength + length
		} else {
			return nil, errors.New("invalid RLP encoding: unexpected byte")
		}
	}

	return nil, errors.New("field index out of range")
}

// ExtractToAddressFromRLP extracts the "to" address from RLP-encoded Dynamic Fee (EIP-1559) transaction bytes.
//
// The "to" address is the sixth field (index 5) in the RLP list:
// [chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList]
//
// Parameters:
// - txBytes: A byte slice containing the RLP-encoded EIP-1559 transaction (prefixed with tx type 0x02).
//
// Returns:
// - []byte: The "to" address (20 bytes), or empty if the transaction is a contract creation.
// - error: An error if the input bytes are invalid or the RLP encoding is malformed.
func ExtractToAddressFromRLP(txBytes []byte) ([]byte, error) {
	if len(txBytes) == 0 {
		return nil, errors.New("empty transaction bytes")
	}

	txType := txBytes[0]
	if txType != 0x02 {
		return nil, errors.New("unsupported transaction type: only EIP-1559 (type 0x02) is supported")
	}

	txBytes = txBytes[1:]

	offset, err := parseRLPList(txBytes)
	if err != nil {
		return nil, err
	}

	toBytes, err := extractRLPField(txBytes, offset, 5)
	if err != nil {
		return nil, err
	}

	return toBytes, nil
}

// ExtractToAddressFromRLPZk extracts the "to" address from an RLP-encoded Dynamic Fee (EIP-1559) transaction
// in a ZK circuit context.
//
// The "to" address is the sixth field (index 5) in the RLP list:
// [chainId(0), nonce(1), maxPriorityFeePerGas(2), maxFeePerGas(3), gasLimit(4), to(5), ...]
//
// The function handles variable-length fields according to RLP encoding rules and
// supports addresses up to 20 bytes.
func ExtractToAddressFromRLPZk(api frontend.API, rawTx []frontend.Variable) frontend.Variable {
	offset := getRLPListDataOffsetZk(api, rawTx)

	// Skip fields 0-4: chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit
	for fieldIdx := 0; fieldIdx < 5; fieldIdx++ {
		offset = skipRLPFieldZk(api, rawTx, offset)
	}

	// Extract "to" address (field index 5)
	toAddress := extractRLPFieldValueZk(api, rawTx, offset)

	return toAddress
}

// getValueAtOffset retrieves a value from an array at a dynamic offset in a ZK circuit context.
// Since ZK circuits cannot use dynamic array indexing directly, this function iterates over
// all possible indices and uses conditional selection to return the value at the matching offset.
func getValueAtOffset(api frontend.API, array []frontend.Variable, offset frontend.Variable) frontend.Variable {
	// Initialize the selected value to zero
	selectedValue := frontend.Variable(0)

	// Iterate over all possible indices in the array
	for i := 0; i < len(array); i++ {
		// Check if the current index matches the offset
		isMatch := api.IsZero(api.Sub(offset, frontend.Variable(i)))

		// Select the value at the current index if it matches the offset
		selectedValue = api.Select(isMatch, array[i], selectedValue)
	}

	return selectedValue
}

// bytesToUint64 converts a byte slice to a uint64.
// - RLP encodes integers as big-endian byte arrays.
// - This function reconstructs the integer by processing the bytes in order.
// Steps:
// 1. Iterate through the byte slice.
// 2. Shift the result left by 8 bits and OR it with the current byte.

func bytesToUint64(b []byte) uint64 {
	var result uint64
	for _, byte := range b {
		result = (result << 8) | uint64(byte)
	}
	return result
}
