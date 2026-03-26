package compiler

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/localcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/specialqueries"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/stitchsplit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// ArcaneParams is an option for the Arcane compiler
type ArcaneParams func(*arcaneParamSet)

// arcaneParamSet collects optional parameters for the Arcane compiler.
type arcaneParamSet struct {
	minStickSize             int
	targetColSize            int
	withLogs                 bool
	WithoutMpts              bool
	debugMode                bool
	name                     string
	innerProductMinimalRound int
	genCSVAfterExpansion     string
}

// MaybeWith allows conditionally activating an option if the condition is true.
func MaybeWith(condition bool, option ArcaneParams) ArcaneParams {
	return func(set *arcaneParamSet) {
		if condition {
			option(set)
		}
	}
}

// WithStitcherMinSize sets the minimum size for the stitcher. All columns
// under this size are moved to public columns.
func WithStitcherMinSize(minStickSize int) ArcaneParams {
	return func(set *arcaneParamSet) {
		set.minStickSize = minStickSize
	}
}

// WithTargetColSize sets the target size for the columns.
func WithTargetColSize(targetColSize int) ArcaneParams {
	return func(set *arcaneParamSet) {
		set.targetColSize = targetColSize
	}
}

// WithLogs tells the compiler to logs compilation stats.
func WithLogs() ArcaneParams {
	return func(set *arcaneParamSet) {
		set.withLogs = true
	}
}

// WithoutMpts tells the compiler to skip the Mpts compiler.
func WithoutMpts() ArcaneParams {
	return func(set *arcaneParamSet) {
		set.WithoutMpts = true
	}
}

// WithDebugMode tells the compiler to run in debug mode. It
// will sanity-check the prover as it is generating the proof
// to help identify which are the queries that are incorrect
// and stop immediately.
func WithDebugMode(name string) ArcaneParams {
	return func(set *arcaneParamSet) {
		set.debugMode = true
		set.name = name
	}
}

// WithInnerProductMinimalRound sets the minimum round for the inner product compiler.
func WithInnerProductMinimalRound(minRound int) ArcaneParams {
	return func(set *arcaneParamSet) {
		set.innerProductMinimalRound = minRound
	}
}

// GenCSVAfterExpansion tells the compiler to generate a CSV file containing all
// the column informations after the expansion. The provided string is the path
// where to write the CSV file.
func GenCSVAfterExpansion(genCSVAfterExpansion string) ArcaneParams {
	return func(set *arcaneParamSet) {
		set.genCSVAfterExpansion = genCSVAfterExpansion
	}
}

// Arcane is a grouping of all compilers. It compiles
// any wizard into a single-point polynomial-IOP
func Arcane(options ...ArcaneParams) func(comp *wizard.CompiledIOP) {

	params := &arcaneParamSet{}
	for _, op := range options {
		op(params)
	}

	if params.minStickSize == 0 {
		params.minStickSize = 256
	}

	return func(comp *wizard.CompiledIOP) {
		if params.withLogs {
			logdata.Log("initially")(comp)
		}

		specialqueries.RangeProof(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "-range-proof"))(comp)
		}

		specialqueries.CompileFixedPermutations(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "fixed-permutations"))(comp)
		}

		permutation.CompileViaGrandProduct(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "grand-product"))(comp)
		}

		logderivativesum.CompileLookups(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "lookups"))(comp)
		}

		horner.CompileProjection(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "projection"))(comp)
		}

		// Note: when the option is not passed to Arcane, the value of the
		// minimal round is zero, which is the exact same as when not passing
		// the option at all to the inner-product compiler.
		innerproduct.Compile(innerproduct.WithMinimalRound(params.innerProductMinimalRound))(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "innerproduct"))(comp)
		}

		if params.withLogs {
			logdata.Log("after-expansion")(comp)
		}

		if len(params.genCSVAfterExpansion) > 0 {
			logdata.GenCSV(files.MustOverwrite(params.genCSVAfterExpansion), logdata.IncludeAllFilter)(comp)
		}

		stitchsplit.Stitcher(params.minStickSize, params.targetColSize)(comp)
		stitchsplit.Splitter(params.targetColSize)(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "stitch-split"))(comp)
		}

		if params.withLogs {
			logdata.Log("post-rectangularization")(comp)
		}

		cleanup.CleanUp(comp)
		localcs.Compile(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "localcs"))(comp)
		}

		globalcs.Compile(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "globalcs"))(comp)
		}

		univariates.Naturalize(comp)
		if params.debugMode {
			dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "naturalize"))(comp)
		}

		if !params.WithoutMpts {
			mpts.Compile()(comp)
			if params.debugMode {
				dummy.CompileAtProverLvl(dummy.WithMsg(params.name + "mpts"))(comp)
			}
		}

		if params.withLogs {
			logdata.Log("end-of-arcane")(comp)
		}
	}
}
