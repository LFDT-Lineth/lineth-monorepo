package execution

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
)

type Witness struct {
	FuncInp *public_input.Execution
	ZkEVM   *zkevm.Witness
}

func Prove(cfg *config.Config, req *Request, large bool) (*Response, error) {
	traces := &cfg.TracesLimits
	if large {
		traces = &cfg.TracesLimitsLarge
	}

	var resp Response

	// TODO @gbotrel wrap profiling in the caller; so that we can properly return errors
	profiling.ProfileTrace("execution",
		cfg.Debug.Profiling,
		cfg.Debug.Tracing,
		func() {
			// Compute the prover's output.
			// WARN: CraftProverOutput calls functions that can panic.
			out := CraftProverOutput(cfg, req)

			if cfg.Execution.ProverMode != config.ProverModeProofless {
				// Development, Partial, Full or Full-large Mode

				// WARN:
				// - MustProveAndPass can panic
				// - Execution prover calls function that can panic
				// - NewFromString can panic
				out.Proof, out.VerifyingKeyShaSum = mustProveAndPass(
					cfg,
					traces,
					NewWitness(cfg, req, &out),
				)

				out.Version = cfg.Version
				out.ProverMode = cfg.Execution.ProverMode
				out.VerifierIndex = uint(cfg.Aggregation.VerifierID) // TODO @gbotrel revisit
			} else {
				// Proofless Mode
				// task.ProverConfig.Mode() == config.ProverProofless
				logrus.Infof("Running the prover in proofless mode")
				out.Version = cfg.Version
				out.ProverMode = config.ProverModeProofless
			}

			resp = out
		})

	return &resp, nil
}

// mustProveAndPass the prover (in the void). Does not takes a
// prover-step function performing the assignment but a function
// returning such a function. This is important to avoid side-effects
// when calling it twice.
func mustProveAndPass(
	cfg *config.Config,
	traces *config.TracesLimits,
	w *Witness,
) (proofHexString string, vkeyShaSum string) {

	switch cfg.Execution.ProverMode {
	case config.ProverModeDev, config.ProverModePartial:
		if cfg.Execution.ProverMode == config.ProverModePartial {

			logrus.Info("Running the PARTIAL prover")

			// And run the partial-prover with only the main steps. The generated
			// proof is sanity-checked to ensure that the prover never outputs
			// invalid proofs.
			partial := zkevm.FullZkEVMCheckOnly(traces)
			proof := partial.ProveInner(w.ZkEVM)
			if err := partial.VerifyInner(proof); err != nil {
				utils.Panic("The prover did not pass: %v", err)
			}
		}

		srsProvider, err := circuits.NewSRSStore(cfg.PathForSRS())
		if err != nil {
			utils.Panic(err.Error())
		}

		setup, err := dummy.MakeUnsafeSetup(srsProvider, circuits.MockCircuitIDExecution, ecc.BLS12_377.ScalarField())
		if err != nil {
			utils.Panic(err.Error())
		}

		return dummy.MakeProof(&setup, w.FuncInp.SumAsField(), circuits.MockCircuitIDExecution), setup.VerifyingKeyDigest()

	case config.ProverModeFull:
		logrus.Info("Running the FULL prover")

		// Run the full prover to obtain the intermediate proof
		logrus.Info("Get Full IOP")
		fullZkEvm := zkevm.FullZkEvm(traces)

		var (
			setup       circuits.Setup
			errSetup    error
			chSetupDone = make(chan struct{})
		)
		go func() {
			setup, errSetup = circuits.LoadSetup(cfg, circuits.ExecutionCircuitID)
			close(chSetupDone)
		}()

		// Generates the inner-proof and sanity-check it so that we ensure that
		// the prover nevers outputs invalid proofs.
		proof := fullZkEvm.ProveInner(w.ZkEVM)

		logrus.Info("Sanity-checking the inner-proof")
		if err := fullZkEvm.VerifyInner(proof); err != nil {
			utils.Panic("The prover did not pass: %v", err)
		}

		// wait for setup to be loaded
		<-chSetupDone
		if errSetup != nil {
			utils.Panic("could not load setup: %v", errSetup)
		}

		// ensure the checksum for the traces in the setup matches the one in the config
		setupCfgChecksum, err := setup.Manifest.GetString("cfg_checksum")
		if err != nil {
			utils.Panic("could not get the traces checksum from the setup manifest: %v", err)
		}
		if setupCfgChecksum != traces.Checksum() {
			utils.Panic("traces checksum in the setup manifest does not match the one in the config")
		}

		// TODO: implements the collection of the functional inputs from the prover response
		return execution.MakeProof(traces, setup, fullZkEvm.WizardIOP, proof, *w.FuncInp), setup.VerifyingKeyDigest()

	case config.ProverModeBench:

		// Run the full prover to obtain the intermediate proof
		logrus.Info("Get Full IOP")
		fullZkEvm := zkevm.FullZkEvm(traces)

		// Generates the inner-proof and sanity-check it so that we ensure that
		// the prover nevers outputs invalid proofs.
		proof := fullZkEvm.ProveInner(w.ZkEVM)

		logrus.Info("Sanity-checking the inner-proof")
		if err := fullZkEvm.VerifyInner(proof); err != nil {
			utils.Panic("The prover did not pass: %v", err)
		}
		return "", ""

	case config.ProverModeCheckOnly:

		fullZkEvm := zkevm.FullZkEVMCheckOnly(traces)
		// this will panic to alert errors, so there is no need to handle or
		// sanity-check anything.
		logrus.Infof("Prover starting the prover")
		_ = fullZkEvm.ProveInner(w.ZkEVM)
		logrus.Infof("Prover checks passed")
		return "", ""

	default:
		panic("not implemented")
	}
}
