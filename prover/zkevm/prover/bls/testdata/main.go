package main

import "log/slog"

const (
	path_add_g1    = "bls_g1_add_inputs.csv"
	path_add_g2    = "bls_g2_add_inputs.csv"
	path_msm_g1    = "bls_g1_msm_inputs-%d.csv"
	path_msm_g2    = "bls_g2_msm_inputs-%d.csv"
	path_pairing   = "bls_pairing_inputs-%d.csv"
	path_map_g1    = "bls_g1_map_inputs.csv"
	path_map_g2    = "bls_g2_map_inputs.csv"
	path_pointeval = "bls_pointeval_inputs-%d.csv"
)

const (
	maxNbPairingInputs = 3
	maxNbMsmInputs     = 3
	nbRepetitionsMap   = 1
)

const (
	nbMsmPerOutput       = 100
	nbPairingPerOutput   = 50
	nbPointEvalPerOutput = 50
)

//go:generate go run .
func main() {
	logger := slog.Default()

	logger.Info("Generating test data for BLS operations")
	logger.Info("Generating ADD test data", "path_g1", path_add_g1, "path_g2", path_add_g2)
	if err := mainAdd(); err != nil {
		logger.Error("Failed to generate ADD test data", "error", err)
		return
	}
	logger.Info("Generating MAP test data", "path_g1", path_msm_g1, "path_g2", path_msm_g2)
	if err := mainMsm(); err != nil {
		logger.Error("Failed to generate MSM test data", "error", err)
		return
	}
	logger.Info("Generating PAIRING test data", "path_pairing", path_pairing)
	if err := mainPairing(); err != nil {
		logger.Error("Failed to generate PAIRING test data", "error", err)
		return
	}
	logger.Info("Generating MAP test data", "path_g1", path_map_g1, "path_g2", path_map_g2)
	if err := mainMap(); err != nil {
		logger.Error("Failed to generate MAP test data", "error", err)
		return
	}
	logger.Info("Generating POINT EVALUATION test data", "path_pointeval", path_pointeval)
	if err := mainPointEval(); err != nil {
		logger.Error("Failed to generate POINT EVALUATION test data", "error", err)
		return
	}
	logger.Info("Test data generation completed successfully")
}
