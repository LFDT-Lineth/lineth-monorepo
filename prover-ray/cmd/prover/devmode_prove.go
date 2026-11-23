package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const (
	devmodeHashLen  = 32
	devmodeVKLen    = 32
	devmodeLimbSize = 4
)

// runDevmodeProve serves devmode proofs for the rollup guest's tests and tools. It reads one
// request per line from in: the 32-byte public-input hash and the 32-byte program VK, each as 64
// hex digits, separated by one space. The devmode public input of a request is the 32 bytes of the
// hash, one cell each, followed by the 8 big-endian 32-bit limbs of the VK. For each request it
// writes the proof image encoded for devmode.GuestBase to out, behind its length as 4 big-endian
// bytes. The system is built once, before the first request. A malformed request, or a VK limb that
// is not a canonical KoalaBear element, ends the command with an error.
func runDevmodeProve(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("prover devmode-prove", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments %q", fs.Args())
	}

	sys, err := devmode.NewSystem()
	if err != nil {
		return fmt.Errorf("devmode.NewSystem: %w", err)
	}

	lines := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	for lines.Scan() {
		pub, err := devmodePublicInput(lines.Text())
		if err != nil {
			return err
		}
		image, err := devmodeProve(sys, pub)
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

func devmodeProve(sys *wiop.System, pub wiop.PublicInput) ([]byte, error) {
	proof, provedPub, err := devmode.Prove(sys, pub)
	if err != nil {
		return nil, err
	}
	return devmode.Encode(sys, proof, provedPub, devmode.GuestBase)
}

// devmodePublicInput is the devmode public input of a request line.
func devmodePublicInput(line string) (wiop.PublicInput, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return nil, fmt.Errorf("request %q: want a hash and a VK", line)
	}
	hash, err := hex.DecodeString(fields[0])
	if err != nil || len(hash) != devmodeHashLen {
		return nil, fmt.Errorf("request %q: the hash is not %d bytes of hex", line, devmodeHashLen)
	}
	vk, err := hex.DecodeString(fields[1])
	if err != nil || len(vk) != devmodeVKLen {
		return nil, fmt.Errorf("request %q: the VK is not %d bytes of hex", line, devmodeVKLen)
	}

	pub := make(wiop.PublicInput, 0, devmode.NumPublicInputs)
	for _, b := range hash {
		var e field.Element
		e.SetUint64(uint64(b))
		pub = append(pub, field.ElemFromBase(e))
	}
	modulus := field.Modulus().Uint64()
	for i := 0; i < devmodeVKLen; i += devmodeLimbSize {
		limb := uint64(binary.BigEndian.Uint32(vk[i : i+devmodeLimbSize]))
		if limb >= modulus {
			return nil, fmt.Errorf("request %q: VK limb %d is not a canonical field element", line, i/devmodeLimbSize)
		}
		var e field.Element
		e.SetUint64(limb)
		pub = append(pub, field.ElemFromBase(e))
	}
	return pub, nil
}
