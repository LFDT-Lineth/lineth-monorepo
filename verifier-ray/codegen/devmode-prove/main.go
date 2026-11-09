// Command devmode-prove is a long-lived devmode prover for callers that need many proof images.
//
// It reads one request per line from stdin: the 32-byte public-input hash and the 32-byte program
// VK, each as 64 hex digits, separated by one space. The devmode public input of a request is the
// 32 bytes of the hash, one cell each, followed by the 8 big-endian 32-bit limbs of the VK. For each
// request it writes the proof image encoded for devmode.GuestBase to stdout, behind its length as
// 4 big-endian bytes. The system is built once, before the first request.
//
// A malformed request, or a VK limb that is not a canonical KoalaBear element, ends the process
// with a message on stderr and a nonzero status.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const (
	hashLen  = 32
	vkLen    = 32
	limbSize = 4
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "devmode-prove:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	sys, err := devmode.NewSystem()
	if err != nil {
		return fmt.Errorf("devmode.NewSystem: %w", err)
	}

	lines := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	for lines.Scan() {
		pub, err := publicInput(lines.Text())
		if err != nil {
			return err
		}
		image, err := prove(sys, pub)
		if err != nil {
			return err
		}
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(image)))
		if _, err := writer.Write(length[:]); err != nil {
			return err
		}
		if _, err := writer.Write(image); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return lines.Err()
}

func prove(sys *wiop.System, pub wiop.PublicInput) ([]byte, error) {
	proof, provedPub, err := devmode.Prove(sys, pub)
	if err != nil {
		return nil, err
	}
	return devmode.Encode(sys, proof, provedPub, devmode.GuestBase)
}

// publicInput is the devmode public input of a request line.
func publicInput(line string) (wiop.PublicInput, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return nil, fmt.Errorf("request %q: want a hash and a VK", line)
	}
	hash, err := hex.DecodeString(fields[0])
	if err != nil || len(hash) != hashLen {
		return nil, fmt.Errorf("request %q: the hash is not %d bytes of hex", line, hashLen)
	}
	vk, err := hex.DecodeString(fields[1])
	if err != nil || len(vk) != vkLen {
		return nil, fmt.Errorf("request %q: the VK is not %d bytes of hex", line, vkLen)
	}

	pub := make(wiop.PublicInput, 0, devmode.NumPublicInputs)
	for _, b := range hash {
		var e field.Element
		e.SetUint64(uint64(b))
		pub = append(pub, field.ElemFromBase(e))
	}
	modulus := field.Modulus().Uint64()
	for i := 0; i < vkLen; i += limbSize {
		limb := uint64(binary.BigEndian.Uint32(vk[i : i+limbSize]))
		if limb >= modulus {
			return nil, fmt.Errorf("request %q: VK limb %d is not a canonical field element", line, i/limbSize)
		}
		var e field.Element
		e.SetUint64(limb)
		pub = append(pub, field.ElemFromBase(e))
	}
	return pub, nil
}
