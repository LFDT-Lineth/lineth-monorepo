package arithmetization

import (
	"errors"
	"fmt"

	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/go-corset/pkg/util/field/koalabear"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/sirupsen/logrus"
)

// ReadExpandedTraces parses the provided trace file, expands it and returns the
// corset object holding the expanded traces.
func AssignFromLtTraces(run *wizard.ProverRuntime, schema *air.Schema[koalabear.Element], expTraces trace.Trace[koalabear.Element], moduleLimits *config.TracesLimits) {

	// This loops checks the module assignment to see if we have created a 77
	// error.
	var (
		modules           = expTraces.Modules().Collect()
		err77             error
		maxRatio          = float64(0)
		argMaxRatioLimit  = 0
		argMaxRatioHeight = uint(0)
	)

	for _, module := range modules {

		var (
			name   = module.Name().String()
			limit  = moduleLimits.GetLimit(name)
			height = module.Height()
			ratio  = float64(height) / float64(limit)
			level  = logrus.InfoLevel
		)

		// The arithmetization can give us an unnamed module, so we skip it
		if len(name) == 0 {
			continue
		}

		if maxRatio < ratio {
			maxRatio = ratio
			argMaxRatioLimit = limit
			argMaxRatioHeight = height
		}

		if uint(limit) < height {
			level = logrus.ErrorLevel
			err77 = errors.Join(err77, fmt.Errorf("limit overflow: module '%s' overflows its limit height=%v limit=%v ratio=%v", name, height, limit, ratio))

		}

		logrus.StandardLogger().Logf(level, "module utilization module=%v height=%v limit=%v ratio=%v", name, height, limit, ratio)
	}

	if err77 != nil {
		exit.OnLimitOverflow(argMaxRatioLimit, int(argMaxRatioHeight), err77)
	}
	// Iterate each module of trace
	for modId := range expTraces.Width() {
		var trMod = expTraces.Module(modId)
		// Iterate each column in module
		parallel.Execute(int(trMod.Width()), func(start, stop int) {
			for id := start; id < stop; id++ {

				var (
					col     = trMod.Column(uint(id))
					name    = ifaces.ColID(wizardName(trMod.Name().String(), col.Name()))
					wCol    = run.Spec.Columns.GetHandle(name)
					padding field.Element
					data    = col.Data()
				)

				if !run.Spec.Columns.Exists(name) {
					continue
				}

				plain := make([]field.Element, data.Len())
				for i := range plain {
					var ith field.Element
					plain[i] = *ith.SetBytes(data.Get(uint(i)).Bytes())
				}
				// Configure padding value
				padding.SetBytes(col.Padding().Bytes())
				// Done
				run.AssignColumn(ifaces.ColID(name), smartvectors.LeftPadded(plain, padding, wCol.Size()))
			}
		})
	}

}
