package circuits

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"runtime"

	"github.com/consensys/gnark"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	plonk_bls12377 "github.com/consensys/gnark/backend/plonk/bls12-377"
	plonk_bn254 "github.com/consensys/gnark/backend/plonk/bn254"
	plonk_bw6761 "github.com/consensys/gnark/backend/plonk/bw6-761"
	"github.com/consensys/gnark/backend/solidity"
	"github.com/sirupsen/logrus"

	kzg377 "github.com/consensys/gnark-crypto/ecc/bls12-377/kzg"
	kzg254 "github.com/consensys/gnark-crypto/ecc/bn254/kzg"
	kzgbw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/kzg"

	"github.com/consensys/gnark/constraint"
	gnarkio "github.com/consensys/gnark/io"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	// solidityPragmaVersion is the version of the Solidity compiler to target.
	// it is used for generating the verifier contract from the PLONK verifying key.
	solidityPragmaVersion = "0.8.26"
)

// Setup contains the proving and verifying keys of a circuit, as well as the constraint system.
// It's a common structure used to pass around the resources associated with a circuit.
type Setup struct {
	Manifest     SetupManifest
	Circuit      constraint.ConstraintSystem
	ProvingKey   plonk.ProvingKey // this is not serialized to disk
	VerifyingKey plonk.VerifyingKey
}

// MakeSetup returns a setup for a given circuit that is either downloaded from
// the S3 bucket or generated in an unsafe manner.
func MakeSetup(
	ctx context.Context,
	circuitName CircuitID,
	ccs constraint.ConstraintSystem,
	srsProvider SRSProvider,
	extraFlags map[string]any,
) (Setup, error) {

	srsCanonical, srsLagrange, err := srsProvider.GetSRS(ctx, ccs)
	if err != nil {
		return Setup{}, fmt.Errorf("while fetching the SRS: %w", err)
	}

	pk, vk, err := plonk.Setup(ccs, srsCanonical, srsLagrange)
	if err != nil {
		return Setup{}, fmt.Errorf("while calling gnark's setup function: %w", err)
	}

	setup := Setup{
		ProvingKey:   pk,
		VerifyingKey: vk,
		Circuit:      ccs,
	}

	setup.Manifest = NewSetupManifest(string(circuitName), ccs.GetNbConstraints(), fieldToCurve(ccs.Field()), extraFlags)
	if setup.Manifest.Checksums.VerifyingKey, err = objectChecksum(vk); err != nil {
		return Setup{}, fmt.Errorf("computing checksum for verifying key: %w", err)
	}
	if setup.Manifest.Checksums.Circuit, err = objectChecksum(ccs); err != nil {
		return Setup{}, fmt.Errorf("computing checksum for circuit: %w", err)
	}
	hasSolidity := setup.Circuit.Field().String() == ecc.BN254.ScalarField().String()
	if hasSolidity {
		h := sha256.New()
		if err = vk.ExportSolidity(h, solidity.WithPragmaVersion(solidityPragmaVersion)); err != nil {
			return Setup{}, fmt.Errorf("computing checksum for verifier contract: %w", err)
		}
		setup.Manifest.Checksums.VerifierContract = "0x" + hex.EncodeToString(h.Sum(nil))
	}

	return setup, nil
}

func (s *Setup) CurveID() ecc.ID {
	return fieldToCurve(s.Circuit.Field())
}

func (s *Setup) VerifyingKeyDigest() string {
	r, err := objectChecksum(s.VerifyingKey)
	if err != nil {
		utils.Panic("could not get the verifying key digest: %v", err)
	}
	return r
}

// CircuitDigest computes the SHA256 digest of the circuit.
func CircuitDigest(circuit constraint.ConstraintSystem) (string, error) {
	return objectChecksum(circuit)
}

// WriteTo writes the setup assets to specified root directory.
func (s *Setup) WriteTo(rootDir string) error {
	circuitPath := filepath.Join(rootDir, config.CircuitFileName)
	manifestPath := filepath.Join(rootDir, config.ManifestFileName)
	verifyingKeyPath := filepath.Join(rootDir, config.VerifyingKeyFileName)
	solidityVerifierPath := filepath.Join(rootDir, config.VerifierContractFileName)

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return fmt.Errorf("creating directory %q: %w", rootDir, err)
	}

	if err := s.Manifest.WriteTo(manifestPath); err != nil {
		return fmt.Errorf("writing manifest to file: %w", err)
	}

	if err := writeToFile(circuitPath, s.Circuit); err != nil {
		return fmt.Errorf("writing circuit to file: %w", err)
	}
	if err := writeToFile(verifyingKeyPath, s.VerifyingKey); err != nil {
		return fmt.Errorf("writing verifying key to file: %w", err)
	}

	hasSolidity := s.Circuit.Field().String() == ecc.BN254.ScalarField().String()
	if hasSolidity {
		f, err := os.Create(solidityVerifierPath)
		if err != nil {
			return fmt.Errorf("creating verifier contract file: %w", err)
		}
		defer f.Close()
		if err = s.VerifyingKey.ExportSolidity(f, solidity.WithPragmaVersion(solidityPragmaVersion)); err != nil {
			return fmt.Errorf("exporting verifier contract to file: %w", err)
		}
	}

	return nil
}

func LoadSetup(cfg *config.Config, circuitID CircuitID) (Setup, error) {
	runtime.GC()

	rootDir := cfg.PathForSetup(string(circuitID))
	manifestPath := filepath.Join(rootDir, config.ManifestFileName)
	manifest, err := ReadSetupManifest(manifestPath)
	if err != nil {
		return Setup{}, fmt.Errorf("reading manifest from file: %w", err)
	}

	curveID, err := ecc.IDFromString(manifest.CurveID)
	if err != nil {
		return Setup{}, fmt.Errorf("parsing curve ID: %w", err)
	}

	circuitPath := filepath.Join(rootDir, config.CircuitFileName)
	circuit := plonk.NewCS(curveID)
	if err := readFromFile(circuitPath, circuit); err != nil {
		return Setup{}, fmt.Errorf("reading circuit from file: %w", err)
	}

	verifyingKeyPath := filepath.Join(rootDir, config.VerifyingKeyFileName)
	vk := plonk.NewVerifyingKey(curveID)
	if err := readFromFile(verifyingKeyPath, vk); err != nil {
		return Setup{}, fmt.Errorf("reading verifying key from file: %w", err)
	}

	vkChecksum, err := objectChecksum(vk)
	if err != nil {
		return Setup{}, fmt.Errorf("computing checksum for verifying key: %w", err)
	}

	if vkChecksum != manifest.Checksums.VerifyingKey {
		return Setup{}, fmt.Errorf("verifying key checksum mismatch: expected %q, got %q", manifest.Checksums.VerifyingKey, vkChecksum)
	}

	// Load the proving key from the SRS provider
	srsProvider, err := NewSRSStore(cfg.PathForSRS())
	if err != nil {
		return Setup{}, fmt.Errorf("creating SRS provider: %w", err)
	}
	srsCanonical, srsLagrange, err := srsProvider.GetSRS(context.Background(), circuit)
	if err != nil {
		return Setup{}, fmt.Errorf("fetching SRS: %w", err)
	}
	pk := plonk.NewProvingKey(curveID)
	var kzgVkFromVk, kzgVkFromSrs io.WriterTo
	switch pk := pk.(type) {
	case *plonk_bn254.ProvingKey:
		pk.Vk = vk.(*plonk_bn254.VerifyingKey)
		srsC := srsCanonical.(*kzg254.SRS)
		pk.Kzg = srsC.Pk
		pk.KzgLagrange = srsLagrange.(*kzg254.SRS).Pk
		kzgVkFromVk = &pk.Vk.Kzg
		kzgVkFromSrs = &srsC.Vk
	case *plonk_bls12377.ProvingKey:
		pk.Vk = vk.(*plonk_bls12377.VerifyingKey)
		srsC := srsCanonical.(*kzg377.SRS)
		pk.Kzg = srsC.Pk
		pk.KzgLagrange = srsLagrange.(*kzg377.SRS).Pk
		kzgVkFromVk = &pk.Vk.Kzg
		kzgVkFromSrs = &srsC.Vk
	case *plonk_bw6761.ProvingKey:
		pk.Vk = vk.(*plonk_bw6761.VerifyingKey)
		srsC := srsCanonical.(*kzgbw6.SRS)
		pk.Kzg = srsC.Pk
		pk.KzgLagrange = srsLagrange.(*kzgbw6.SRS).Pk
		kzgVkFromVk = &pk.Vk.Kzg
		kzgVkFromSrs = &srsC.Vk
	default:
		panic("not implemented")
	}

	if err = utils.WriterstoEqual(kzgVkFromSrs, kzgVkFromVk); err != nil {
		return Setup{}, fmt.Errorf("verifying key <> SRS mismatch: %w", err)
	}

	return Setup{
		Manifest:     *manifest,
		Circuit:      circuit,
		ProvingKey:   pk,
		VerifyingKey: vk,
	}, nil
}

func writeToFile(path string, object any) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %q: %w", path, err)
	}
	defer f.Close()

	if err := writeToWriter(f, object); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}

	return nil
}

func writeToWriter(w io.Writer, object any) error {

	// if any implements io.WriterRawTo, we use it, else we use io.WriterTo, else we panic.
	var wFunc func(w io.Writer) (n int64, err error)
	switch v := object.(type) {
	case gnarkio.WriterRawTo:
		wFunc = v.WriteRawTo
	case io.WriterTo:
		wFunc = v.WriteTo
	default:
		panic(fmt.Sprintf("unsupported type %T", object))
	}

	// 512Mb buffer using bufio.Writer
	buf := bufio.NewWriterSize(w, 512*1024*1024)

	if _, err := wFunc(buf); err != nil {
		return err
	}
	return buf.Flush()
}

func ReadVerifyingKey(path string, into plonk.VerifyingKey) error {
	return readFromFile(path, into)
}

// this function is used to read circuits and verifying keys from disk
func readFromFile(path string, into any) error {
	logrus.Debugf("reading %s", path)

	// if any implements io.ReaderRawFrom, we use it, else we use io.ReaderFrom, else we panic.
	var rFunc func(r io.Reader) (n int64, err error)
	switch v := into.(type) {
	case gnarkio.UnsafeReaderFrom:
		rFunc = v.UnsafeReadFrom
	case io.ReaderFrom:
		rFunc = v.ReadFrom
	default:
		panic(fmt.Sprintf("unsupported type %T", into))
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %q: %w", path, err)
	}

	_, err = rFunc(f)
	if err = errors.Join(err, f.Close()); err != nil {
		return fmt.Errorf("reading %q from disk: %w", path, err)
	}

	logrus.Debugf("read %s", path)

	return err
}

func objectChecksum(object any) (string, error) {
	h := sha256.New()
	if err := writeToWriter(h, object); err != nil {
		return "", fmt.Errorf("writing to hasher: %w", err)
	}

	return "0x" + hex.EncodeToString(h.Sum(nil)), nil
}

func fieldToCurve(q *big.Int) ecc.ID {
	curves := make(map[string]ecc.ID)
	for _, c := range gnark.Curves() {
		fHex := c.ScalarField().Text(16)
		curves[fHex] = c
	}
	fHex := q.Text(16)
	curve, ok := curves[fHex]
	if !ok {
		return ecc.UNKNOWN
	}
	return curve
}
