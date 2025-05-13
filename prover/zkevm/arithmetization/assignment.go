package arithmetization

import (
	"errors"
	"fmt"

	"github.com/consensys/go-corset/pkg/air"
	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/sirupsen/logrus"
)

// ReadExpandedTraces parses the provided trace file, expands it and returns the
// corset object holding the expanded traces.
func AssignFromLtTraces(run *wizard.ProverRuntime, schema *air.Schema, expTraces trace.Trace, limits *config.TracesLimits) {

	// This loops checks the module assignment to see if we have created a 77
	// error.
	var (
		modules      = expTraces.Modules().Collect()
		moduleLimits = mapModuleLimits(limits)
		err77        error
		numCols      = expTraces.Width()
	)

	for _, module := range modules {

		var (
			name   = module.Name()
			limit  = moduleLimits[module.Name()]
			height = module.Height()
			ratio  = float64(height) / float64(limit)
			level  = logrus.InfoLevel
		)

		if uint(limit) < height {
			level = logrus.ErrorLevel
			err77 = errors.Join(err77, fmt.Errorf("limit overflow: module '%s' overflows its limit height=%v limit=%v ratio=%v", name, height, limit, ratio))
		}

		logrus.StandardLogger().Logf(level, "module utilization module=%v height=%v limit=%v ratio=%v", name, height, limit, ratio)
	}

	if err77 != nil {
		logrus.Errorf("Error code 77 %v", err77)
		exit.OnLimitOverflow()
	}

	for id := uint(0); id < numCols; id++ {

		var (
			col     = expTraces.Column(id)
			name    = ifaces.ColID(wizardName(getModuleName(schema, col), col.Name()))
			wCol    = run.Spec.Columns.GetHandle(name)
			padding = col.Padding()
			data    = col.Data()
			plain   = make([]field.Element, data.Len())
		)

		if !run.Spec.Columns.Exists(name) {
			continue
		}

		for i := range plain {
			plain[i] = data.Get(uint(i))
		}

		run.AssignColumn(ifaces.ColID(name), smartvectors.LeftPadded(plain, padding, wCol.Size()))
	}
}
