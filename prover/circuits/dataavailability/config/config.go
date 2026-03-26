package config

import (
	"github.com/consensys/linea-monorepo/prover/config"
)

// CircuitSizes is the Data Availability circuit configuration, in accordance with its manifest.
type CircuitSizes struct {
	MaxUncompressedNbBytes int
	MaxNbBatches           int
	DictNbBytes            int
}

// FromGlobalConfig extracts the fields describing the size parameters of
// the data availability circuit from cfg.
func FromGlobalConfig(cfg config.DataAvailability) CircuitSizes {
	return CircuitSizes{
		MaxUncompressedNbBytes: cfg.MaxUncompressedNbBytes,
		MaxNbBatches:           cfg.MaxNbBatches,
		DictNbBytes:            cfg.DictNbBytes,
	}
}
