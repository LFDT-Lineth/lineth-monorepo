// Helper for the Keccak `IN_BYTES` blob expected by rust/src/keccak/keccak.rs.
//
// Reads a JSON-Lines `.accepts` file (one Keccak test vector per line) and emits
// a single `0x<hex>` blob to stdout that concatenates the first N vectors in the
// 720-byte per-vector layout the guest program reads:
//
//	|-- 680 B padded message (BE, u5440 left-padded) --|
//	|--   8 B msg_len_bits   (LE, u64)                --|
//	|--  32 B expected digest (BE)                     --|
//
// Usage:
//
//	keccak_accepts_to_in_bytes <accepts-file> <n-vectors>
//
// Exits non-zero if any line fails validation or fewer than N vectors are available.
package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	messageBytes      = 680 // 5440 bits / 8
	messageLenBytes   = 8
	digestBytes       = 32
	vectorBytes       = messageBytes + messageLenBytes + digestBytes // 720
	messageHexChars   = messageBytes * 2
	digestHexChars    = digestBytes * 2
	maxMessageLenBits = messageBytes * 8
)

type acceptsLine struct {
	MessageLength string `json:"message_length"`
	Message       string `json:"message"`
	Result        string `json:"result"`
}

func decodeHex(s string, want int, lineno int, field string) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		return nil, fmt.Errorf("line %d: %s missing 0x prefix", lineno, field)
	}
	body := s[2:]
	if len(body) != want {
		return nil, fmt.Errorf("line %d: %s hex length is %d, expected %d", lineno, field, len(body), want)
	}
	buf, err := hex.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("line %d: %s hex decode: %w", lineno, field, err)
	}
	return buf, nil
}

func encodeVector(line string, lineno int) ([]byte, error) {
	var obj acceptsLine
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, fmt.Errorf("line %d: json: %w", lineno, err)
	}
	msg, err := decodeHex(obj.Message, messageHexChars, lineno, "message")
	if err != nil {
		return nil, err
	}
	digest, err := decodeHex(obj.Result, digestHexChars, lineno, "result")
	if err != nil {
		return nil, err
	}

	lenStr := obj.MessageLength
	if !strings.HasPrefix(lenStr, "0x") && !strings.HasPrefix(lenStr, "0X") {
		return nil, fmt.Errorf("line %d: message_length missing 0x prefix", lineno)
	}
	msgLenBits, err := strconv.ParseUint(lenStr[2:], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("line %d: message_length parse: %w", lineno, err)
	}
	if msgLenBits > maxMessageLenBits {
		return nil, fmt.Errorf("line %d: message_length=%d exceeds %d", lineno, msgLenBits, maxMessageLenBits)
	}

	out := make([]byte, 0, vectorBytes)
	out = append(out, msg...)
	var lenLE [messageLenBytes]byte
	for i := 0; i < messageLenBytes; i++ {
		lenLE[i] = byte(msgLenBits >> (8 * i))
	}
	out = append(out, lenLE[:]...)
	out = append(out, digest...)
	if len(out) != vectorBytes {
		return nil, fmt.Errorf("line %d: assembled vector is %d bytes, expected %d", lineno, len(out), vectorBytes)
	}
	return out, nil
}

func run(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: keccak_accepts_to_in_bytes <accepts-file> <n-vectors>")
	}
	path := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil || n <= 0 {
		return fmt.Errorf("n-vectors must be a positive integer, got %q", args[1])
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	blob := make([]byte, 0, n*vectorBytes)
	scanner := bufio.NewScanner(f)
	// Long lines: a single accepts line is ~1500 chars but allow ample headroom.
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<22)
	lineno := 0
	emitted := 0
	for scanner.Scan() && emitted < n {
		lineno++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		vec, err := encodeVector(line, lineno)
		if err != nil {
			return err
		}
		blob = append(blob, vec...)
		emitted++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	if emitted < n {
		return fmt.Errorf("%s: only %d vector(s) available, requested %d", path, emitted, n)
	}

	if _, err := fmt.Fprint(os.Stdout, "0x", hex.EncodeToString(blob), "\n"); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	fmt.Fprintf(os.Stderr, "emitted %d vector(s) -> %d bytes (%d hex chars)\n",
		emitted, len(blob), len(blob)*2)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
