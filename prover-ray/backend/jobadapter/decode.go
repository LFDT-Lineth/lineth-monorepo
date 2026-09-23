package jobadapter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// JSON decode helpers shared by the rollup and aggregation request decoders.
// They take an op label so each decoder's errors name themselves. request.go
// keeps its own copies for the L2-execution path.

func decErrf(op, format string, args ...any) error {
	return fmt.Errorf(op+": "+format, args...)
}

func fieldOf(m map[string]json.RawMessage, key, op, prefix string) (json.RawMessage, error) {
	v, ok := m[key]
	if !ok {
		return nil, decErrf(op, "missing %s%s", prefix, key)
	}
	return v, nil
}

func objectOf(raw json.RawMessage, op, ctx string) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, decErrf(op, "%s must be an object: %w", ctx, err)
	}
	if obj == nil {
		return nil, decErrf(op, "%s must be an object", ctx)
	}
	return obj, nil
}

func arrayOf(raw json.RawMessage, op, ctx string) ([]json.RawMessage, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, decErrf(op, "%s must be an array: %w", ctx, err)
	}
	if arr == nil {
		return nil, decErrf(op, "%s must be an array", ctx)
	}
	return arr, nil
}

func stringOf(raw json.RawMessage, op, ctx string) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", decErrf(op, "%s must be a string: %w", ctx, err)
	}
	return s, nil
}

func boolOf(raw json.RawMessage, op, ctx string) (bool, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, decErrf(op, "%s must be a boolean: %w", ctx, err)
	}
	return b, nil
}

// uint64Of accepts a JSON number or a 0x-hex string.
func uint64Of(raw json.RawMessage, op, ctx string) (uint64, error) {
	var n uint64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && strings.HasPrefix(s, "0x") {
		if v, err := strconv.ParseUint(s[2:], 16, 64); err == nil {
			return v, nil
		}
	}
	return 0, decErrf(op, "%s must be a uint64 (number or 0x-hex), got %s", ctx, raw)
}

func hexBytesOf(raw json.RawMessage, op, ctx string) ([]byte, error) {
	s, err := stringOf(raw, op, ctx)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(s, "0x") {
		return nil, decErrf(op, "%s must be a 0x-prefixed hex string", ctx)
	}
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return nil, decErrf(op, "%s: invalid hex: %w", ctx, err)
	}
	return b, nil
}

func fixedHexOf(raw json.RawMessage, op, ctx string, wantBytes int) ([]byte, error) {
	b, err := hexBytesOf(raw, op, ctx)
	if err != nil {
		return nil, err
	}
	if len(b) != wantBytes {
		return nil, decErrf(op, "%s must be %d bytes, got %d", ctx, wantBytes, len(b))
	}
	return b, nil
}

func hash32Of(raw json.RawMessage, op, ctx string) ([32]byte, error) {
	var out [32]byte
	b, err := fixedHexOf(raw, op, ctx, hashByteSize)
	if err != nil {
		return out, err
	}
	copy(out[:], b)
	return out, nil
}

func addressOf(raw json.RawMessage, op, ctx string) ([20]byte, error) {
	var out [20]byte
	b, err := fixedHexOf(raw, op, ctx, addressByteSize)
	if err != nil {
		return out, err
	}
	copy(out[:], b)
	return out, nil
}

// Field lookup plus conversion.

func getObject(m map[string]json.RawMessage, key, op, prefix string) (map[string]json.RawMessage, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return nil, err
	}
	return objectOf(raw, op, prefix+key)
}

func getArray(m map[string]json.RawMessage, key, op, prefix string) ([]json.RawMessage, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return nil, err
	}
	return arrayOf(raw, op, prefix+key)
}

func getU64(m map[string]json.RawMessage, key, op, prefix string) (uint64, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return 0, err
	}
	return uint64Of(raw, op, prefix+key)
}

func getBool(m map[string]json.RawMessage, key, op, prefix string) (bool, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return false, err
	}
	return boolOf(raw, op, prefix+key)
}

func getHexBytes(m map[string]json.RawMessage, key, op, prefix string) ([]byte, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return nil, err
	}
	return hexBytesOf(raw, op, prefix+key)
}

func getFixedHex(m map[string]json.RawMessage, key, op, prefix string, wantBytes int) ([]byte, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return nil, err
	}
	return fixedHexOf(raw, op, prefix+key, wantBytes)
}

func getHash32(m map[string]json.RawMessage, key, op, prefix string) ([32]byte, error) {
	raw, err := fieldOf(m, key, op, prefix)
	if err != nil {
		return [32]byte{}, err
	}
	return hash32Of(raw, op, prefix+key)
}

func hash32List(raw []json.RawMessage, op, prefix string) ([][32]byte, error) {
	out := make([][32]byte, len(raw))
	for i, r := range raw {
		h, err := hash32Of(r, op, fmt.Sprintf("%s[%d]", prefix, i))
		if err != nil {
			return nil, err
		}
		out[i] = h
	}
	return out, nil
}

func addressList(raw []json.RawMessage, op, prefix string) ([][20]byte, error) {
	out := make([][20]byte, len(raw))
	for i, r := range raw {
		a, err := addressOf(r, op, fmt.Sprintf("%s[%d]", prefix, i))
		if err != nil {
			return nil, err
		}
		out[i] = a
	}
	return out, nil
}
