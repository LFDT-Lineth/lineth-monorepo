package main

import (
	"bytes"
	"crypto/elliptic"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"slices"
	"strconv"

	cryptoecdsa "crypto/ecdsa"

	"github.com/consensys/gnark-crypto/ecc/secp256r1"
	fp_secp256r1 "github.com/consensys/gnark-crypto/ecc/secp256r1/fp"
	fr_secp256r1 "github.com/consensys/gnark-crypto/ecc/secp256r1/fr"
)

const (
	outputFile = "p256verify_inputs.csv"
	// get from https://eips.ethereum.org/assets/eip-7951/test-vectors.json
	testvectors = "test-vectors.json"
)

//go:generate go run main.go
func main() {
	logger := slog.Default()
	fout, err := os.Create(outputFile)
	if err != nil {
		logger.Error("failed to create output file", "error", err)
		return
	}
	defer fout.Close()

	ftc, err := os.Open(testvectors)
	if err != nil {
		logger.Error("failed to open test vectors file", "error", err)
		return
	}
	defer ftc.Close()

	w := csv.NewWriter(fout)
	defer w.Flush()
	if err := w.Write([]string{
		"ID",
		"DATA_P256_VERIFY_FLAG",
		"RSLT_P256_VERIFY_FLAG",
		"INDEX",
		"LIMB",
		"CIRCUIT_SELECTOR_P256_VERIFY",
	}); err != nil {
		logger.Error("failed to write csv headers", "error", err)
		return
	}
	var testcases []vector
	dec := json.NewDecoder(ftc)
	if err := dec.Decode(&testcases); err != nil {
		logger.Error("failed to decode test vectors", "error", err)
		return
	}
	var id int
	for _, tc := range testcases {
		isValid, h, r, s, qx, qy, err := mockedArithmetization(&tc)
		if errors.Is(err, errInputSplit) {
			logger.Debug("skipping test case due to input error", "id", tc.Name)
			continue
		}
		tcVerified := expectedBool(tc.Expected)
		if err != nil && tcVerified {
			// arithmetization said test case failed, but test case indicates success
			logger.Error("supposed successful verification but arithmetization failed: %v", err)
			return
		}
		if isValid != tcVerified {
			logger.Error("mismatch between arithmetization and test case", "id", tc.Name, "arithmetization", isValid, "testcase", tcVerified)
			return
		}
		if err != nil {
			logger.Debug("skipping test case as not going to circuit")
			continue
		}
		toCircuit := "1"
		hLimbs := splitBigToLimbs(h)
		rLimbs := splitBigToLimbs(r)
		sLimbs := splitBigToLimbs(s)
		qxLimbs := splitBigToLimbs(qx)
		qyLimbs := splitBigToLimbs(qy)
		dataLimbs := slices.Concat(
			hLimbs,
			rLimbs,
			sLimbs,
			qxLimbs,
			qyLimbs,
		)
		resLimbs := []string{"0"}
		if isValid {
			resLimbs = append(resLimbs, "1")
		} else {
			resLimbs = append(resLimbs, "0")
		}
		for i, limb := range dataLimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),
				"1",
				"0",
				strconv.Itoa(i),
				limb,
				toCircuit,
			}); err != nil {
				logger.Error("failed to write csv record", "error", err)
				return
			}
		}
		for i, limb := range resLimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),
				"0",
				"1",
				strconv.Itoa(i),
				limb,
				toCircuit,
			}); err != nil {
				logger.Error("failed to write csv record", "error", err)
				return
			}
		}
		id++
	}
}

func splitBigToLimbs(s *big.Int) []string {
	var sb [32]byte
	s.FillBytes(sb[:])
	limbs := []string{
		fmt.Sprintf("0x%x", sb[0:16]),
		fmt.Sprintf("0x%x", sb[16:32]),
	}
	return limbs
}

type vector struct {
	Name     string `json:"Name,omitempty"`
	Input    string `json:"Input"`
	Expected string `json:"Expected"`
}

// mockedArithmetization performs the checks what the arithmetization would
// perform. It returns a boolean indicating if the signature is valid or not and
// and an error if the test case would fail already at the arithmetization
// level.
func mockedArithmetization(testcase *vector) (isValid bool, h, r, s, qx, qy *big.Int, err error) {
	// arithmetization checks:
	// * Input length: Input MUST be exactly 160 bytes !!!!
	// * Signature component bounds: Both r and s MUST satisfy 0 < r < n and 0 < s < n !!!
	// * Public key bounds: Both qx and qy MUST satisfy 0 ≤ qx < p and 0 ≤ qy < p !!!
	// * Point validity: The point (qx, qy) MUST satisfy the curve equation qy^2 ≡ qx^3 + a*qx + b (mod p) !!!
	// * Point not at infinity: The point (qx, qy) MUST NOT be the point at infinity (represented as (0, 0)) !!!

	// 1. first check the input length:
	h, r, s, qx, qy, err = splitInput160(testcase.Input)
	if err != nil {
		return
	}
	// 2. check signature component bounds
	modFr := fr_secp256r1.Modulus()
	if r.Cmp(big.NewInt(0)) != 1 || r.Cmp(modFr) != -1 {
		err = errors.New("r out of bounds")
		return
	}
	if s.Cmp(big.NewInt(0)) != 1 || s.Cmp(modFr) != -1 {
		err = errors.New("s out of bounds")
		return
	}
	// 3. check public key bounds
	modFp := fp_secp256r1.Modulus()
	if qx.Cmp(big.NewInt(0)) == -1 || qx.Cmp(modFp) != -1 {
		err = errors.New("qx out of bounds")
		return
	}
	if qy.Cmp(big.NewInt(0)) == -1 || qy.Cmp(modFp) != -1 {
		err = errors.New("qy out of bounds")
		return
	}
	// 4. check that point is on the curve
	var P secp256r1.G1Affine
	P.X.SetBigInt(qx)
	P.Y.SetBigInt(qy)
	if !P.IsOnCurve() {
		err = errors.New("point not on curve")
		return
	}
	// 5. check that point is not at infinity
	if P.IsInfinity() {
		err = errors.New("point at infinity")
		return
	}
	// if we reached this point, all arithmetization checks passed. Now check
	// signature validity
	pk := cryptoecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     qx,
		Y:     qy,
	}
	msgBytes := h.Bytes()
	ok := cryptoecdsa.Verify(&pk, msgBytes, r, s)
	if !ok {
		return
	}
	isValid = true
	return
}

var errInputSplit = errors.New("input error")

func splitInput160(hexInput string) (h, r, s, qx, qy *big.Int, err error) {
	var raw []byte
	raw, err = hex.DecodeString(hexInput)
	if err != nil {
		err = errInputSplit
		// invalid hex encoding
		return
	}
	if len(raw) != 160 {
		// invalid length
		err = errInputSplit
		return
	}
	h = new(big.Int).SetBytes(raw[0:32])
	r = new(big.Int).SetBytes(raw[32:64])
	s = new(big.Int).SetBytes(raw[64:96])
	qx = new(big.Int).SetBytes(raw[96:128])
	qy = new(big.Int).SetBytes(raw[128:160])
	return
}

func expectedBool(s string) bool {
	raw, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	one := make([]byte, 32)
	one[31] = 1
	return bytes.Equal(raw, one)
}
