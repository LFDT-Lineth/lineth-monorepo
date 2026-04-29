package main

import (
	"math/big"
	"math/rand"
	"testing"

	circuitInvalidity "github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/spf13/viper"
)

// test uses the Dev mode of the prover
func TestInvalidityBadNonceBalance(t *testing.T) {

	// Create a reproducible RNG
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewSource(seed))

	configFile = "../../../../config/config-integration-development.toml"
	viper.Set("assets_dir", "../../../../prover-assets")

	for _, invalidityType := range []circuitInvalidity.InvalidityType{circuitInvalidity.BadNonce, circuitInvalidity.BadBalance} {

		spec := &InvalidityProofSpec{
			ChainID:             big.NewInt(51),
			ExpectedBlockHeight: 1_000_000_000,
			FtxNumber:           1678,
			InvalidityType:      &invalidityType,
		}

		ProcessInvaliditySpec(rng, spec, nil, "spec-invalidity.json")
	}
}

// test uses the Partial mode of the prover
func TestInvalidityFilteredAddress(t *testing.T) {

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewSource(seed))

	configFile = "../../../../config/config-devnet-full.toml"
	viper.Set("assets_dir", "../../../../prover-assets")

	for _, invalidityType := range []circuitInvalidity.InvalidityType{
		circuitInvalidity.FilteredAddressFrom,
		circuitInvalidity.FilteredAddressTo,
	} {
		invType := invalidityType
		t.Run(invType.String(), func(t *testing.T) {
			spec := &InvalidityProofSpec{
				ChainID:             big.NewInt(51),
				ExpectedBlockHeight: 1_000_000_000,
				FtxNumber:           1678,
				InvalidityType:      &invType,
			}

			ProcessInvaliditySpec(rng, spec, nil, "spec-invalidity-filtered.json")
		})
	}
}

// test the bad precompile and too many logs cases only for Dev mod as the other modes require conflated execution traces and zk state merkle proofs.
func TestInvalidityBadPrecompile(t *testing.T) {
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rng := rand.New(rand.NewSource(seed))

	configFile = "../../../../config/config-integration-development.toml"
	viper.Set("assets_dir", "../../../../prover-assets")

	for _, invalidityType := range []circuitInvalidity.InvalidityType{
		circuitInvalidity.BadPrecompile,
		circuitInvalidity.TooManyLogs,
	} {
		invType := invalidityType
		t.Run(invType.String(), func(t *testing.T) {
			spec := &InvalidityProofSpec{
				ChainID:             big.NewInt(51),
				ExpectedBlockHeight: 1_000_000_000,
				FtxNumber:           1678,
				InvalidityType:      &invType,
			}

			ProcessInvaliditySpec(rng, spec, nil, "spec-invalidity-precompile.json")
		})
	}
}
