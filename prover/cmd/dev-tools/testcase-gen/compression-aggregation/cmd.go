package main

// DCC (Dynamic Chain Configuration) version of the testcase generator.
// This is a temporary tool to support contract CI with DCC-enabled verifier contracts.
// Revert this change before the DCC release.

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
	"github.com/spf13/cobra"
)

// Main command of the CLI tool
var rootCmd = &cobra.Command{
	Use:   "generate-dcc",
	Short: "Generates testcase samples with DCC (Dynamic Chain Configuration) support",
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
	AssetsDir: "./prover-assets",
	// Layer2 fields will be populated from the DCC spec
}

func init() {
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
		// #nosec G404 --we don't need a cryptographic RNG for testing purpose
		rng                     = rand.New(rand.NewSource(seed))
		runningSpec             = &AggregationSpec{}
		blobSubmissionResponses = []*blobsubmission.Response{}
		aggregationResponses    *DCCAggregationResponse

		emptyBlobSubmissionSpec BlobSubmissionSpec
		emptyAggSpec            AggregationSpec
	)

	for _, specFile := range specFiles {

		spec := parseSpecFile(specFile)

		hasBlobSubmission := !reflect.DeepEqual(spec.BlobSubmissionSpec, emptyBlobSubmissionSpec)
		hasAggregation := !reflect.DeepEqual(spec.AggregationSpec, emptyAggSpec)

		switch {
		case hasBlobSubmission && !hasAggregation:
			resp := ProcessBlobSubmissionSpec(
				rng, runningSpec, blobSubmissionResponses,
				spec.BlobSubmissionSpec,
			)
			blobSubmissionResponses = append(blobSubmissionResponses, resp)

		case hasAggregation && !hasBlobSubmission:
			if len(blobSubmissionResponses) == 0 {
				printlnAndExit("provided aggregation spec without any blob submission spec before")
			}
			if aggregationResponses != nil {
				printlnAndExit("more than one aggregation spec is not allowed")
			}

			// Apply DCC from the aggregation spec
			applyDCCToConfig(&spec.DynamicChainConfigurationSpec)

			resp := ProcessAggregationSpecDCC(
				rng,
				blobSubmissionResponses[0],
				runningSpec,
				spec.AggregationSpec,
			)
			aggregationResponses = resp

		case hasAggregation && hasBlobSubmission:
			if aggregationResponses != nil {
				printlnAndExit("more than one aggregation spec is not allowed")
			}

			respA := ProcessBlobSubmissionSpec(
				rng, runningSpec, blobSubmissionResponses,
				spec.BlobSubmissionSpec,
			)
			blobSubmissionResponses = append(blobSubmissionResponses, respA)

			// Apply DCC from the aggregation spec
			applyDCCToConfig(&spec.DynamicChainConfigurationSpec)

			resp := ProcessAggregationSpecDCC(
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

	// Write blob submission files
	for i, resp := range blobSubmissionResponses {
		p := path.Join(odir, blobSubmissionRespFileName(resp))
		f, err := os.Create(p)
		if err != nil {
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

	// Write aggregation file with DCC fields
	if aggregationResponses != nil {
		p := path.Join(odir, aggregationRespFileName(blobSubmissionResponses))
		f, err := os.Create(p)
		if err != nil {
			printlnAndExit("Unexpected error creating output file: %v", err)
		}

		// Create the DCC response structure for JSON output
		dccResp := createDCCResponseJSON(aggregationResponses)
		serialized, err := json.MarshalIndent(dccResp, "", "\t")
		if err != nil {
			printlnAndExit("Unexpected error writing output file: %v", err)
		}

		_, err = f.Write(serialized)
		if err != nil {
			printlnAndExit("Unexpected error writing output file: %v", err)
		}
		f.Close()

		dumpVerifierContract(odir, circuits.MockCircuitIDEmulation)
	}
}

// DCCResponseJSON is the JSON structure for the DCC aggregation response
// Field order matches the expected output from main branch
type DCCResponseJSON struct {
	FinalShnarf                             string   `json:"finalShnarf"`
	ParentAggregationFinalShnarf            string   `json:"parentAggregationFinalShnarf"`
	AggregatedProof                         string   `json:"aggregatedProof"`
	AggregatedProverVersion                 string   `json:"aggregatedProverVersion"`
	AggregatedVerifierIndex                 int      `json:"aggregatedVerifierIndex"`
	AggregatedProofPublicInput              string   `json:"aggregatedProofPublicInput"`
	DataHashes                              []string `json:"dataHashes"`
	DataParentHash                          string   `json:"dataParentHash"`
	ParentStateRootHash                     string   `json:"parentStateRootHash"`
	ParentAggregationLastBlockTimestamp     uint     `json:"parentAggregationLastBlockTimestamp"`
	LastFinalizedBlockNumber                uint     `json:"lastFinalizedBlockNumber"`
	FinalTimestamp                          uint     `json:"finalTimestamp"`
	FinalBlockNumber                        uint     `json:"finalBlockNumber"`
	LastFinalizedL1RollingHash              string   `json:"lastFinalizedL1RollingHash"`
	L1RollingHash                           string   `json:"l1RollingHash"`
	LastFinalizedL1RollingHashMessageNumber uint     `json:"lastFinalizedL1RollingHashMessageNumber"`
	L1RollingHashMessageNumber              uint     `json:"l1RollingHashMessageNumber"`
	L2MerkleRoots                           []string `json:"l2MerkleRoots"`
	L2MerkleTreesDepth                      uint     `json:"l2MerkleTreesDepth"`
	L2MessagingBlocksOffsets                string   `json:"l2MessagingBlocksOffsets"`
}

func createDCCResponseJSON(resp *DCCAggregationResponse) *DCCResponseJSON {
	return &DCCResponseJSON{
		FinalShnarf:                             resp.FinalShnarf,
		ParentAggregationFinalShnarf:            resp.ParentAggregationFinalShnarf,
		AggregatedProof:                         resp.AggregatedProof,
		AggregatedProverVersion:                 resp.AggregatedProverVersion,
		AggregatedVerifierIndex:                 resp.AggregatedVerifierIndex,
		AggregatedProofPublicInput:              resp.AggregatedProofPublicInput,
		DataHashes:                              resp.DataHashes,
		DataParentHash:                          resp.DataParentHash,
		ParentStateRootHash:                     resp.ParentStateRootHash,
		ParentAggregationLastBlockTimestamp:     resp.ParentAggregationLastBlockTimestamp,
		LastFinalizedBlockNumber:                resp.LastFinalizedBlockNumber,
		FinalTimestamp:                          resp.FinalTimestamp,
		FinalBlockNumber:                        resp.FinalBlockNumber,
		LastFinalizedL1RollingHash:              resp.LastFinalizedL1RollingHash,
		L1RollingHash:                           resp.L1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: resp.LastFinalizedL1RollingHashMessageNumber,
		L1RollingHashMessageNumber:              resp.L1RollingHashMessageNumber,
		L2MerkleRoots:                           resp.L2MerkleRoots,
		L2MerkleTreesDepth:                      resp.L2MsgTreesDepth,
		L2MessagingBlocksOffsets:                resp.L2MessagingBlocksOffsets,
	}
}

func printlnAndExit(msg string, args ...any) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Printf(msg, args...)
	os.Exit(1)
}

func validateParameters(cmd *cobra.Command, _ []string) {
	if len(specFiles) == 0 {
		cmd.Usage()
		printlnAndExit("No spec files provided")
	}

	fi, err := os.Lstat(odir)
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

func ProcessBlobSubmissionSpec(
	rng *rand.Rand,
	runningSpec *AggregationSpec,
	prevBlobSubs []*blobsubmission.Response,
	spec BlobSubmissionSpec,
) (resp *blobsubmission.Response) {

	if len(prevBlobSubs) > 0 && !spec.IgnoreBefore {
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

	runningSpec.DataHashes = append(runningSpec.DataHashes, resp.DataHash)
	runningSpec.FinalShnarf = resp.ExpectedShnarf

	if len(prevBlobSubs) > 0 {
		runningSpec.ParentStateRootHash = resp.ParentStateRootHash
		runningSpec.DataParentHash = resp.DataParentHash
	}

	upb := resp.ConflationOrder.UpperBoundaries
	runningSpec.FinalBlockNumber = uint(upb[len(upb)-1])

	return resp
}

// ProcessAggregationSpecDCC processes aggregation spec with DCC support
func ProcessAggregationSpecDCC(
	rng *rand.Rand,
	firstBlobSub *blobsubmission.Response,
	runningSpec *AggregationSpec,
	spec AggregationSpec,
) (resp *DCCAggregationResponse) {

	if !spec.IgnoreBefore {
		spec.DataParentHash = firstBlobSub.DataParentHash
		spec.DataHashes = runningSpec.DataHashes
		spec.FinalShnarf = runningSpec.FinalShnarf
		spec.ParentStateRootHash = firstBlobSub.ParentStateRootHash
		spec.LastFinalizedBlockNumber = uint(firstBlobSub.ConflationOrder.StartingBlockNumber) - 1
		spec.FinalBlockNumber = runningSpec.FinalBlockNumber
		spec.ParentAggregationFinalShnarf = firstBlobSub.PrevShnarf

		if len(runningSpec.LastFinalizedL1RollingHash) > 0 {
			spec.LastFinalizedL1RollingHashMessageNumber = runningSpec.L1RollingHashMessageNumber
			spec.LastFinalizedL1RollingHash = runningSpec.L1RollingHash
		}
	}

	collectedFields := RandAggregation(rng, spec)

	// Call the standard CraftResponse
	baseResp, err := aggregation.CraftResponse(cfg, collectedFields)
	if err != nil {
		printlnAndExit("Could not craft aggregation response : %s", err)
	}

	// Wrap with DCC fields
	resp = wrapResponseWithDCC(
		baseResp,
		collectedFields.LastFinalizedL1RollingHash,
		collectedFields.LastFinalizedL1RollingHashMessageNumber,
	)

	// Recalculate public input hash with DCC (includes chain config hash)
	// and regenerate the proof for that public input
	recalculateDCCPublicInputAndProof(resp)

	// Post-processing
	runningSpec.LastFinalizedL1RollingHash = resp.L1RollingHash
	runningSpec.LastFinalizedL1RollingHashMessageNumber = resp.L1RollingHashMessageNumber

	return resp
}

func blobSubmissionRespFileName(resp *blobsubmission.Response) string {
	start, end := resp.ConflationOrder.Range()
	return fmt.Sprintf("./blocks-%v-%v.json", start, end)
}

func aggregationRespFileName(resp []*blobsubmission.Response) string {
	start, _ := resp[0].ConflationOrder.Range()
	_, end := resp[len(resp)-1].ConflationOrder.Range()
	return fmt.Sprintf("./aggregatedProof-%v-%v.json", start, end)
}

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

	if err := pp.VerifyingKey.ExportSolidity(f, solidity.WithPragmaVersion("0.8.33")); err != nil {
		printlnAndExit("could not export verifying key to solidity: %v", err)
	}
}
