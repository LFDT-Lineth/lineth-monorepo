package blobdecompression

import (
	"bytes"
	"encoding/base64"
	"fmt"
	blob_v0 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	blob_v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/blobdecompression"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"

	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
)

// Generates a concrete proof for the decompression of the blob
func Prove(cfg *config.Config, req *Request) (*Response, error) {

	// Parsing / validating the request
	blobBytes, err := base64.StdEncoding.DecodeString(req.CompressedData)
	if err != nil {
		return nil, fmt.Errorf("could not parse the compressed data: %w", err)
	}

	var (
		xBytes [32]byte
		y      fr381.Element
	)

	if b, err := utils.HexDecodeString(req.ExpectedX); err != nil {
		return nil, fmt.Errorf("could not parse the bytes of the expected x: %w", err)
	} else {
		copy(xBytes[:], b)
	}

	yBytes, err := utils.HexDecodeString(req.ExpectedY)
	if err != nil {
		return nil, fmt.Errorf("could not parse the bytes of the expected y: %w", err)
	}
	y.SetBytes(yBytes)

	// First of all, we need to identify which setup-info to use
	version := blob.GetVersion(blobBytes)
	var (
		circuitID                    circuits.CircuitID
		expectedMaxUsableBytes       int
		expectedMaxUncompressedBytes int
	)
	switch version {
	case 0:
		circuitID = circuits.BlobDecompressionV0CircuitID
		expectedMaxUsableBytes = blob_v0.MaxUsableBytes
		expectedMaxUncompressedBytes = blob_v0.MaxUncompressedBytes
	case 1:
		circuitID = circuits.BlobDecompressionV1CircuitID
		expectedMaxUsableBytes = blob_v1.MaxUsableBytes
		expectedMaxUncompressedBytes = blob_v1.MaxUncompressedBytes
	default:
		return nil, fmt.Errorf("unsupported blob version: %v", version)
	}

	logrus.Info("reading dictionaries")

	dictStore := cfg.BlobDecompressionDictStore(string(circuitID))

	// This computes the assignment

	logrus.Infof("computing the circuit's assignment")

	snarkHash, err := utils.HexDecodeString(req.SnarkHash)
	if err != nil {
		return nil, fmt.Errorf("could not parse the snark hash: %w", err)
	}

	assignment, pubInput, _snarkHash, err := blobdecompression.Assign(
		utils.RightPad(blobBytes, expectedMaxUsableBytes),
		dictStore,
		req.Eip4844Enabled,
		xBytes,
		y,
	)

	if err != nil {
		return nil, fmt.Errorf("while generating the assignment: %w", err)
	}

	if !bytes.Equal(snarkHash, _snarkHash) {
		return nil, fmt.Errorf("blob checksum does not match the one computed by the assigner")
	}

	var (
		setup           circuits.Setup
		proofSerialized string
	)

	if cfg.BlobDecompression.ProverMode == config.ProverModeDev {
		// create a dummy proof instead

		srsProvider, err := circuits.NewSRSStore(cfg.PathForSRS())
		if err != nil {
			return nil, fmt.Errorf("could not create the SRS store: %w", err)
		}

		if setup, err = dummy.MakeUnsafeSetup(srsProvider, circuits.MockCircuitIDDecompression, ecc.BLS12_377.ScalarField()); err != nil {
			return nil, fmt.Errorf("could not make the setup: %w", err)
		}

		proofSerialized = dummy.MakeProof(&setup, pubInput, circuits.MockCircuitIDDecompression)
	} else {
		if setup, err = circuits.LoadSetup(cfg, circuitID); err != nil {
			return nil, fmt.Errorf("could not load the setup: %w", err)
		}

		maxUsableBytes, err := setup.Manifest.GetInt("maxUsableBytes")
		if err != nil {
			return nil, fmt.Errorf("missing maxUsableBytes in the setup manifest: %w", err)
		}

		maxUncompressedBytes, err := setup.Manifest.GetInt("maxUncompressedBytes")
		if err != nil {
			return nil, fmt.Errorf("missing maxUncompressedBytes in the setup manifest: %w", err)
		}

		if maxUsableBytes != expectedMaxUsableBytes {
			return nil, fmt.Errorf("invalid maxUsableBytes in the setup manifest: %v, expected %v", maxUsableBytes, expectedMaxUsableBytes)
		}

		if maxUncompressedBytes != expectedMaxUncompressedBytes {
			return nil, fmt.Errorf("invalid maxUncompressedBytes in the setup manifest: %v, expected %v", maxUncompressedBytes, expectedMaxUncompressedBytes)
		}

		// This section reads the public parameters. This is a time-consuming part
		// of the process.

		opts := []any{
			emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
			emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
		}

		// This actually runs the compression prover

		logrus.Infof("running the decompression prover")

		proof, err := circuits.ProveCheck(
			&setup,
			assignment,
			opts...,
		)

		if err != nil {
			return nil, fmt.Errorf("while generating the proof: %w", err)
		}

		proofSerialized = circuits.SerializeProofRaw(proof)
	}

	logrus.Infof("prover successful : generated proof `%++v` for public input `%v`", proofSerialized, pubInput.String())

	resp := &Response{
		Request:            *req,
		ProverVersion:      cfg.Version,
		DecompressionProof: proofSerialized,
		VerifyingKeyShaSum: setup.VerifyingKeyDigest(),
	}

	resp.Debug.PublicInput = "0x" + pubInput.Text(16)

	return resp, nil
}
