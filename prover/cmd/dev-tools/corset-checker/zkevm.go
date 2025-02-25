package main

import (
	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
)

var globalArith *arithmetization.Arithmetization

func MakeDefine(cfg *config.Config, optConfig *mir.OptimisationConfig) wizard.DefineFunc {
	return func(build *wizard.Builder) {
		globalArith = arithmetization.NewArithmetization(build, arithmetization.Settings{
			Limits:            &cfg.TracesLimits,
			OptimisationLevel: optConfig,
		})

	}
}

func MakeProver(traceFile string) wizard.ProverStep {
	return func(run *wizard.ProverRuntime) {
		globalArith.Assign(run, traceFile)
	}
}
