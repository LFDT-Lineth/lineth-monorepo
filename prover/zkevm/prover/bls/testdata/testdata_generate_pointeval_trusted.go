package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	kzg_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
)

// copied from gnark/std/evmprecompiles/10-kzg_point_evaluation_test.go.
// The trusted setup is not exported there as it is only used in tests. Current package is
// not imported elsewhere, so the included trusted setup is not embedded in the binary.

const (
	// locTrustedSetup is the location of the trusted setup file for the KZG precompile.
	locTrustedSetup = "kzg_trusted_setup.json"
)

var srsPk *kzg_bls12381.ProvingKey
var srsVk *kzg_bls12381.VerifyingKey

func init() {
	f, err := os.Open(locTrustedSetup)
	if err != nil {
		panic(fmt.Sprintf("failed to open trusted setup file: %v", err))
	}
	defer f.Close()
	setup, err := parseTrustedSetup(f)
	if err != nil {
		panic(fmt.Sprintf("failed to parse trusted setup: %v", err))
	}
	srsPk, err = setup.toProvingKey()
	if err != nil {
		panic(fmt.Sprintf("failed to convert trusted setup to proving key: %v", err))
	}
	srsVk, err = setup.toVerifyingKey()
	if err != nil {
		panic(fmt.Sprintf("failed to convert trusted setup to verifying key: %v", err))
	}
}

// trustedSetupJSON represents the trusted setup for the KZG precompile. It is
// used to verify the KZG commitments and openings. The setup is available at
// https://github.com/ethereum/go-ethereum/blob/master/crypto/kzg4844/trusted_setup.json.
// It was generated during the KZG Ceremony.
type trustedSetupJSON struct {
	G1         []string `json:"g1_monomial"`
	G1Lagrange []string `json:"g1_lagrange"`
	G2         []string `json:"g2_monomial"`
}

// parseTrustedSetup reads the trusted setup from the given reader and returns a
// trustedSetupJSON struct. It validates the setup to ensure it has the correct
// number of elements in G1, G2, and G1Lagrange. The G1 and G1Lagrange arrays
// must have exactly `evmBlockSize` elements, while G2 must have at least 2
// elements (but in practice has more for future extensibility). If the setup is
// invalid, it returns an error.
func parseTrustedSetup(r io.Reader) (*trustedSetupJSON, error) {
	var setup trustedSetupJSON
	dec := json.NewDecoder(r)
	if err := dec.Decode(&setup); err != nil {
		return nil, fmt.Errorf("decode trusted setup: %w", err)
	}
	if len(setup.G1) == 0 || len(setup.G2) == 0 || len(setup.G1Lagrange) == 0 {
		return nil, fmt.Errorf("invalid trusted setup: missing G1 or G2 or G1Lagrange")
	}
	if len(setup.G1) != evmBlockSize || len(setup.G1Lagrange) != evmBlockSize {
		return nil, fmt.Errorf("invalid trusted setup: G1 must have %d elements, got %d", evmBlockSize, len(setup.G1))
	}
	return &setup, nil
}

// toProvingKey converts the trusted setup JSON to a ProvingKey for allowing to
// compute the commitment and opening proof.
func (t *trustedSetupJSON) toProvingKey() (*kzg_bls12381.ProvingKey, error) {
	pk := kzg_bls12381.ProvingKey{
		G1: make([]bls12381.G1Affine, len(t.G1)),
	}
	for i, g1 := range t.G1 {
		decoded, err := decodePrefixed(g1)
		if err != nil {
			return nil, fmt.Errorf("decode G1 element %d: %w", i, err)
		}
		nbDec, err := pk.G1[i].SetBytes(decoded)
		if err != nil {
			return nil, fmt.Errorf("set G1 element %d: %w", i, err)
		}
		if nbDec != len(decoded) {
			return nil, fmt.Errorf("set G1 element %d: expected %d bytes, got %d", i, len(decoded), nbDec)
		}
	}
	return &pk, nil
}

// toVerifyingKey converts the trusted setup JSON to a VerifyingKey for allowing
// to verify the opening proof.
func (t *trustedSetupJSON) toVerifyingKey() (*kzg_bls12381.VerifyingKey, error) {
	var vk kzg_bls12381.VerifyingKey
	if len(t.G2) < 2 {
		return nil, fmt.Errorf("invalid trusted setup: G2 must have at least 2 elements")
	}
	if len(t.G1) < 1 {
		return nil, fmt.Errorf("invalid trusted setup: G1 must have at least 1 element")
	}
	decoded, err := decodePrefixed(t.G1[0])
	if err != nil {
		return nil, fmt.Errorf("decode G1 element 0: %w", err)
	}
	nbDec, err := vk.G1.SetBytes(decoded)
	if err != nil {
		return nil, fmt.Errorf("set G1 element 0: %w", err)
	}
	if nbDec != len(decoded) {
		return nil, fmt.Errorf("set G1 element 0: expected %d bytes, got %d", len(decoded), nbDec)
	}
	for i := range 2 {
		decoded, err := decodePrefixed(t.G2[i])
		if err != nil {
			return nil, fmt.Errorf("decode G2 element %d: %w", i, err)
		}
		nbDec, err := vk.G2[i].SetBytes(decoded)
		if err != nil {
			return nil, fmt.Errorf("set G2 element %d: %w", i, err)
		}
		if nbDec != len(decoded) {
			return nil, fmt.Errorf("set G2 element %d: expected %d bytes, got %d", i, len(decoded), nbDec)
		}
		vk.Lines[i] = bls12381.PrecomputeLines(vk.G2[i])
	}
	return &vk, nil
}

func decodePrefixed(line string) ([]byte, error) {
	if !strings.HasPrefix(line, "0x") {
		return nil, fmt.Errorf("invalid prefix in line: %s", line)
	}
	decoded, err := hex.DecodeString(line[2:])
	if err != nil {
		return nil, fmt.Errorf("decode hex string: %w", err)
	}
	return decoded, nil
}
