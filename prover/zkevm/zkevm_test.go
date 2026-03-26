package zkevm

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/config"
)

func TestCanGenerateFullZkEVM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping heavy test in short mode")
	}

	cfg, err := config.NewConfigFromFileUnchecked("../config/config-mainnet-limitless.toml")
	if err != nil {
		t.Fatal(err)
	}

	_ = FullZkEvm(&cfg.TracesLimits, cfg)
}
