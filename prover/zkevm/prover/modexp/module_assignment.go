package modexp

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// antichamberAssignment is a builder structure used to incrementally compute
// the assignment of the column of the [Module] module.
type antichamberAssignment struct {
	isActive    *common.VectorBuilder
	isSmall     *common.VectorBuilder
	isLarge     *common.VectorBuilder
	limbs       *common.VectorBuilder
	toSmallCirc *common.VectorBuilder
}

// Assign assigns the anti-chamber module
func (mod *Module) Assign(run *wizard.ProverRuntime) {

	mod.Input.assignIsModexp(run)

	var (
		modexpCountSmall int = 0
		modexpCountLarge int = 0
		isModexp             = mod.Input.IsModExp.GetColAssignment(run).IntoRegVecSaveAlloc()
		limbs                = mod.Input.Limbs.GetColAssignment(run).IntoRegVecSaveAlloc()
		settings             = mod.Input.Settings
		builder              = antichamberAssignment{
			isActive:    common.NewVectorBuilder(mod.IsActive),
			isSmall:     common.NewVectorBuilder(mod.IsSmall),
			isLarge:     common.NewVectorBuilder(mod.IsLarge),
			limbs:       common.NewVectorBuilder(mod.Limbs),
			toSmallCirc: common.NewVectorBuilder(mod.ToSmallCirc),
		}
	)

	for currPosition := 0; currPosition < len(limbs); {

		if isModexp[currPosition].IsZero() {
			currPosition++
			continue
		}

		// This sanity-check is purely defensive and will indicate that we
		// missed the start of a Modexp instance
		if len(limbs)-currPosition < modexpNumRowsPerInstance {
			utils.Panic("A new modexp is starting but there is not enough rows (currPosition=%v len(ecdata.Limb)=%v)", currPosition, len(limbs))
		}

		isLarge := false

		// An instance is considered large if any of the operand has more than
		// 2 16-bytes limbs.
		for k := 0; k < modexpNumRowsPerInstance; k++ {
			if k%nbLargeModexpLimbs < (nbLargeModexpLimbs-2) && !limbs[currPosition+k].IsZero() {
				isLarge = true
				break
			}
		}

		if isLarge {
			modexpCountLarge++
		} else {
			modexpCountSmall++
		}

		for k := 0; k < modexpNumRowsPerInstance; k++ {

			builder.isActive.PushOne()
			builder.isSmall.PushBoolean(!isLarge)
			builder.isLarge.PushBoolean(isLarge)
			builder.limbs.PushField(limbs[currPosition+k])

			if !isLarge && k%nbLargeModexpLimbs >= (nbLargeModexpLimbs-2) {
				builder.toSmallCirc.PushOne()
			} else {
				builder.toSmallCirc.PushZero()
			}
		}

		currPosition += modexpNumRowsPerInstance
	}

	if modexpCountSmall > settings.MaxNbInstance256 {
		exit.OnLimitOverflow(
			settings.MaxNbInstance256,
			modexpCountSmall,
			fmt.Errorf("limit overflow: the modexp (%d bits) count is %v and the limit is %v", smallModexpSize, modexpCountSmall, settings.MaxNbInstance256),
		)
	}

	if modexpCountLarge > settings.MaxNbInstanceLarge {
		exit.OnLimitOverflow(
			settings.MaxNbInstanceLarge,
			modexpCountLarge,
			fmt.Errorf("limit overflow: the modexp (%d bits) count is %v and the limit is %v", largeModexpSize, modexpCountLarge, settings.MaxNbInstanceLarge),
		)
	}

	builder.isActive.PadAndAssign(run, field.Zero())
	builder.isSmall.PadAndAssign(run, field.Zero())
	builder.isLarge.PadAndAssign(run, field.Zero())
	builder.limbs.PadAndAssign(run, field.Zero())
	builder.toSmallCirc.PadAndAssign(run, field.Zero())

	// It is possible to not declare the circuit (for testing purpose) in that
	// case we skip the corresponding assignment part.
	if mod.GnarkCircuitConnector256Bits != nil {
		mod.GnarkCircuitConnector256Bits.Assign(run)
	}
	if mod.GnarkCircuitConnectorLarge != nil {
		mod.GnarkCircuitConnectorLarge.Assign(run)
	}
}
