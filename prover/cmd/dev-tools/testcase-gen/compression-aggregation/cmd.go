package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/solidity"
	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/backend/blobsubmission"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

// Main command of the CLI tool
var rootCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates a sequence of testcases samples for decompression proof",
	Run:   genFiles,
}

// List of spec to use to generate the testcases
var (
	specFiles []string
	odir      string
	seed      int64
)

// Sample config to use to generate mocked aggregation
var cfg = &config.Config{
	Version: "test",
	Aggregation: config.Aggregation{
		ProverMode: config.ProverModeDev,
		VerifierID: 1,
	},
	AssetsDir: "./prover-assets", // TODO @gbotrel untested
	// Layer2 fields will be populated from the DCC spec in aggregation spec files
}

func init() {
	// Initialize the flags

	rootCmd.Flags().StringArrayVar(
		&specFiles, "spec", nil,
		"JSON spec file to use to generate a sequence of blobs",
	)

	rootCmd.Flags().StringVar(
		&odir, "odir", ".",
		"Output directory where to write the sample files",
	)

	rootCmd.Flags().Int64Var(
		&seed, "seed", 0,
		"Seed to use for the randomness generation",
	)
}

func genFiles(cmd *cobra.Command, args []string) {

	validateParameters(cmd, args)

	var (
		// Create a reproducible RNG
		// #nosec G404 --we don't need a cryptographic RNG for testing purpose
		rng = rand.New(rand.NewSource(seed))
		// Running spec object that accumulate all the intermediate results
		runningSpec = &AggregationSpec{}
		// List of the generated blob submission responses
		blobSubmissionResponses = []*blobsubmission.Response{}
		// Note: there can be at most one aggregation response per sample
		// generation.
		aggregationResponses *aggregation.Response

		// Empty spec, handy for testing if a spec is for aggregation or blob
		// submission.
		emptyBlobSubmissionSpec BlobSubmissionSpec
		emptyAggSpec            AggregationSpec
	)

	for _, specFile := range specFiles {

		spec := parseSpecFile(specFile)

		hasBlobSubmission := !reflect.DeepEqual(spec.BlobSubmissionSpec, emptyBlobSubmissionSpec)
		hasAggregation := !reflect.DeepEqual(spec.AggregationSpec, emptyAggSpec)

		switch {
		// It's a blob submission spec
		case hasBlobSubmission && !hasAggregation:
			resp := ProcessBlobSubmissionSpec(
				rng, runningSpec, blobSubmissionResponses,
				spec.BlobSubmissionSpec,
			)
			blobSubmissionResponses = append(blobSubmissionResponses, resp)

		// It's an aggregation spec
		case hasAggregation && !hasBlobSubmission:

			if len(blobSubmissionResponses) == 0 {
				printlnAndExit("provided aggregation spec without any blob submission spec before")
			}

			if aggregationResponses != nil {
				printlnAndExit("more than one aggregation spec is not allowed")
			}

			// Apply DCC from the aggregation spec to the global config
			applyDCCToConfig(&spec.DynamicChainConfigurationSpec)

			resp := ProcessAggregationSpec(
				rng,
				blobSubmissionResponses[0],
				runningSpec,
				spec.AggregationSpec,
			)
			aggregationResponses = resp

		// It's both, in that case, we do first the submission then the aggregation
		case hasAggregation && hasBlobSubmission:

			if aggregationResponses != nil {
				printlnAndExit("more than one aggregation spec is not allowed")
			}

			respA := ProcessBlobSubmissionSpec(
				rng, runningSpec, blobSubmissionResponses,
				spec.BlobSubmissionSpec,
			)
			blobSubmissionResponses = append(blobSubmissionResponses, respA)

			// Apply DCC from the aggregation spec to the global config
			applyDCCToConfig(&spec.DynamicChainConfigurationSpec)

			resp := ProcessAggregationSpec(
				rng,
				blobSubmissionResponses[0],
				runningSpec,
				spec.AggregationSpec,
			)
			aggregationResponses = resp

		default:
			printlnAndExit("Spec is neither a BlobSubmissionSpec nor an AggregationSpec : %++v", spec)
		}
	}

	// Then write the files
	for i, resp := range blobSubmissionResponses {
		p := path.Join(odir, blobSubmissionRespFileName(resp))
		f, err := os.Create(p)
		if err != nil {
			// It should not be possible due to the above check
			printlnAndExit("Unexpected error creating output file: %v", err)
		}

		serialized, err := json.MarshalIndent(blobSubmissionResponses[i], "", "\t")
		if err != nil {
			printlnAndExit("could not serialize submission response: %v", err)
		}

		_, err = f.Write(serialized)
		if err != nil {
			printlnAndExit("Unexpected error writing output file: %v", err)
		}

		f.Close()
	}

	// and for the aggregation
	if aggregationResponses != nil {
		p := path.Join(odir, aggregationRespFileName(blobSubmissionResponses))
		f, err := os.Create(p)
		if err != nil {
			// It should not be possible due to the above check
			printlnAndExit("Unexpected error creating output file: %v", err)
		}

		serialized, err := json.MarshalIndent(aggregationResponses, "", "\t")
		if err != nil {
			printlnAndExit("Unexpected error writing output file: %v", err)
		}

		_, err = f.Write(serialized)
		if err != nil {
			printlnAndExit("Unexpected error writing output file: %v", err)
		}

		f.Close()

		// and dump the circuit id
		dumpVerifierContract(odir, circuits.MockCircuitIDEmulation)
	}
}

// applyDCCToConfig applies the Dynamic Chain Configuration (DCC) from the aggregation spec
// to the global config. DCC contains chain-specific parameters (chainID, baseFee, coinBase,
// L2MessageServiceAddr) that are used when computing the aggregation public input hash.
// These values must match between the prover and the on-chain verifier contract.
func applyDCCToConfig(dccSpec *DynamicChainConfigurationSpec) {
	if dccSpec == nil {
		printlnAndExit("aggregation spec must include dynamicChainConfigurationSpec")
	}
	cfg.Layer2.ChainID = uint(dccSpec.ChainID)
	cfg.Layer2.BaseFee = uint(dccSpec.BaseFee)
	cfg.Layer2.CoinBaseStr = dccSpec.CoinBase
	cfg.Layer2.CoinBase = common.HexToAddress(dccSpec.CoinBase)
	cfg.Layer2.MsgSvcContractStr = dccSpec.L2MessageServiceAddr
	cfg.Layer2.MsgSvcContract = common.HexToAddress(dccSpec.L2MessageServiceAddr)
}

func printlnAndExit(msg string, args ...any) {
	// Ensures the new line
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Printf(msg, args...)
	os.Exit(1)
}

// Validates the parameters and exit the program if this is unsuccessful.
func validateParameters(cmd *cobra.Command, _ []string) {

	if len(specFiles) == 0 {
		cmd.Usage()
		printlnAndExit("No spec files provided")
	}

	fi, err := os.Lstat(odir)

	// Try to create the directory if it doesn't exist
	if os.IsNotExist(err) {
		err = os.MkdirAll(odir, 0755)
		if err != nil {
			printlnAndExit("could not create directory %v : %w", odir, err)
		}
		fi, err = os.Lstat(odir)
	}

	if err != nil {
		printlnAndExit("could not lstat odir %v: %w", odir, err)
	}

	if !fi.IsDir() {
		printlnAndExit("odir %v is not a directory", odir)
	}
}

// Parse a spec file from a file name and a dir. Exit if an error occurs.
func parseSpecFile(file string) RandGenSpec {

	spec := RandGenSpec{}
	f, err := os.Open(file)
	if err != nil {
		printlnAndExit("Could not open spec file %v : %w\n", file, err)
	}

	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		printlnAndExit("Could not parse spec file %v : %w\n", file, err)
	}

	return spec
}

// Process a blob submission spec file
func ProcessBlobSubmissionSpec(
	rng *rand.Rand,
	runningSpec *AggregationSpec,
	prevBlobSubs []*blobsubmission.Response,
	spec BlobSubmissionSpec,
) (
	resp *blobsubmission.Response,
) {

	// Override the relevant fields using the previously generated response
	// so that we create a sequence.
	if len(prevBlobSubs) > 0 && !spec.IgnoreBefore {

		// Start on top of the previous response
		prevBlobSubmissionResp := prevBlobSubs[len(prevBlobSubs)-1]
		upBs := prevBlobSubmissionResp.ConflationOrder.UpperBoundaries
		spec.StartFromL2Block = upBs[len(upBs)-1] + 1
		spec.ParentShnarf = prevBlobSubmissionResp.ExpectedShnarf
		spec.ParentZkRootHash = prevBlobSubmissionResp.FinalStateRootHash
		spec.ParentDataHash = prevBlobSubmissionResp.DataHash
	}

	req := RandBlobSubmission(rng, spec)
	resp, err := blobsubmission.CraftResponse(req)
	if err != nil {
		printlnAndExit("Could not craft blob submission response : %s", err)
	}

	// Collect the elements of the response that can be used later if we build
	// an aggregation response sample.
	runningSpec.DataHashes = append(runningSpec.DataHashes, resp.DataHash)
	// overrides everytime to get the last value at the end
	runningSpec.FinalShnarf = resp.ExpectedShnarf

	// only for the first blob submission
	if len(prevBlobSubs) > 0 {
		runningSpec.ParentStateRootHash = resp.ParentStateRootHash
		runningSpec.DataParentHash = resp.DataParentHash
	}

	upb := resp.ConflationOrder.UpperBoundaries
	runningSpec.FinalBlockNumber = uint(upb[len(upb)-1])

	return resp
}

// Process an aggregation spec file
func ProcessAggregationSpec(
	rng *rand.Rand,
	firstBlobSub *blobsubmission.Response,
	runningSpec *AggregationSpec,
	spec AggregationSpec,
) (
	resp *aggregation.Response,
) {

	// Applies the fields that were collected earlier while processing the
	// blob submissions. So that the aggregation job is consecutive to them.
	if !spec.IgnoreBefore {
		spec.DataParentHash = firstBlobSub.DataParentHash
		spec.DataHashes = runningSpec.DataHashes
		spec.FinalShnarf = runningSpec.FinalShnarf
		spec.ParentStateRootHash = firstBlobSub.ParentStateRootHash
		spec.LastFinalizedBlockNumber = uint(firstBlobSub.ConflationOrder.StartingBlockNumber) - 1
		spec.FinalBlockNumber = runningSpec.FinalBlockNumber
		spec.ParentAggregationFinalShnarf = firstBlobSub.PrevShnarf

		// Not for the first aggregation that we generate in a row
		if len(runningSpec.LastFinalizedL1RollingHash) > 0 {
			spec.LastFinalizedL1RollingHashMessageNumber = runningSpec.L1RollingHashMessageNumber
			spec.LastFinalizedL1RollingHash = runningSpec.L1RollingHash
		}
	}

	collectedFields := RandAggregation(rng, spec)

	resp, err := aggregation.CraftResponse(cfg, collectedFields)
	if err != nil {
		printlnAndExit("Could not craft aggregation response : %s", err)
	}

	// Post-processing, stores the L1ROllingHash data
	runningSpec.LastFinalizedL1RollingHash = resp.L1RollingHash
	runningSpec.LastFinalizedL1RollingHashMessageNumber = resp.L1RollingHashMessageNumber

	return resp
}

// returns the filename of a blob compression spec file
func blobSubmissionRespFileName(resp *blobsubmission.Response) string {
	start, end := resp.ConflationOrder.Range()
	return fmt.Sprintf("./blocks-%v-%v.json", start, end)
}

// returns the filename of an aggregation spec file, the name is derived from
// the first and the last name of the blob compression spec files.
func aggregationRespFileName(resp []*blobsubmission.Response) string {
	start, _ := resp[0].ConflationOrder.Range()
	_, end := resp[len(resp)-1].ConflationOrder.Range()
	return fmt.Sprintf("./aggregatedProof-%v-%v.json", start, end)
}

// dump the verifier contract in odir
func dumpVerifierContract(odir string, circID circuits.MockCircuitID) {
	filepath := filepath.Join(odir, fmt.Sprintf("Verifier%v.sol", circID))
	f := files.MustOverwrite(filepath)
	defer f.Close()

	srsProvider, err := circuits.NewSRSStore(cfg.PathForSRS())
	if err != nil {
		printlnAndExit("could not create SRS provider: %v", err)
	}

	pp, err := dummy.MakeUnsafeSetup(srsProvider, circID, ecc.BN254.ScalarField())
	if err != nil {
		printlnAndExit("could not create public parameters: %v", err)
	}

	if err := pp.VerifyingKey.ExportSolidity(f, solidity.WithPragmaVersion("0.8.26")); err != nil {
		printlnAndExit("could not export verifying key to solidity: %v", err)
	}
}
