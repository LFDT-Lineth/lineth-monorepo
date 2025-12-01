package plonkinternal

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// Compile-time sanity check the satisfaction of the interface RangeChecker by
// externalRangeChecker
var _ frontend.Rangechecker = &externalRangeChecker{}

// externalRangeChecker wraps the frontend.Builder. We require that the builder
// also implements [frontend.Committer].
//
// The range checking gadget in gnark works by checking the capabilities of the
// builder and if it provides `native` range checking capabilities (by
// implementing [frontend.Rangechecker]), uses it instead of doing range checks
// using a lookup table. By default the builder doesn't implement the interface
// and thus uses the fallback (using lookup tables). But within Wizard IOP we
// can defer range checking to the Wizard IOP instead of doing in PLONK circuit
// in gnark. For that, the [externalRangeChecker] implements
// [frontend.Rangechecker] by providing [externalRangeChecker.Check] method.
//
// Currently, the implementation is dummy as this wrapped builder doesn't
// actually pass the variables to range check on to Wizard IOP, but ideally it
// should. But this most probably requires that we tag the variables internally
// and then reorder them in the PLONK solver. There, we should probably mark
// these variables using a custom gate which allows later to map these variables
// into Wizard column which can be range checked.
type externalRangeChecker struct {
	storeCommitBuilder
	checked              []frontend.Variable
	rcCols               chan [][2]int
	addGateForRangeCheck bool
}

// storeCommitBuilder implements [frontend.Builder], [frontend.Committer] and
// [kvstore.Store].
type storeCommitBuilder interface {
	frontend.Builder[constraint.U64]
	frontend.Committer
	SetKeyValue(key, value any)
	GetKeyValue(key any) (value any)
	GetWireConstraints(wires []frontend.Variable, addMissing bool) ([][2]int, error)
}

// newExternalRangeChecker takes compiled IOP and returns [frontend.NewBuilder].
// The returned constructor can be passed to [frontend.Compile] to instantiate a
// new builder of constraint system.
//
// The function also returns an rcGetter which is in substance a function to be
// called after the compilation to return the position of the wires that are
// range-checked in the circuit.
//
// Example usage:
//
//	```
//	builder, rcGetter := newExternalRangeChecker(comp)
//	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField, circuit, builder)
//	if err != nil {
//		return fmt.Errorf("could not compile because: %w", err)
//	}
//
//	// This returns the position of the wires to range-check.
//	checkedWires := rcGetter()
//	```
func newExternalRangeChecker(addGateForRangeCheck bool) (frontend.NewBuilder, func() [][2]int) {
	rcCols := make(chan [][2]int)
	return func(field *big.Int, config frontend.CompileConfig) (frontend.Builder[constraint.U64], error) {
			b, err := scs.NewBuilder[constraint.U64](field, config)
			if err != nil {
				return nil, fmt.Errorf("could not create new native builder: %w", err)
			}
			scb, ok := b.(storeCommitBuilder)
			if !ok {
				return nil, fmt.Errorf("native builder doesn't implement committer or kvstore")
			}
			return &externalRangeChecker{
				storeCommitBuilder:   scb,
				rcCols:               rcCols,
				addGateForRangeCheck: addGateForRangeCheck,
			}, nil
		}, func() [][2]int {
			return <-rcCols
		}
}

// Check implements [frontend.RangeChecker]
func (builder *externalRangeChecker) Check(v frontend.Variable, bits int) {

	// This applies specifically for the Sha2 circuit which generates range-
	// checks for constants integers. When that happens, we skip the range-check:
	if checkIfConst(v, bits) {
		return
	}

	// we store the ID of the wire we want to range check. Later, when calling
	// [Compile], we pass all the wires to the [GetWireGates] function of the
	// underlying builder to get the locations of the constraints
	builder.checked = append(builder.checked, v)
}

// Compile processes range checked variables and then calls Compile method of
// the underlying builder.
func (builder *externalRangeChecker) Compile() (constraint.ConstraintSystem, error) {
	// GetWireGates may add gates if [addGateForRangeCheck] is true. Call it
	// synchronously before calling compile on the circuit.
	cols, err := builder.storeCommitBuilder.GetWireConstraints(builder.checked, builder.addGateForRangeCheck)
	if err != nil {
		return nil, fmt.Errorf("get wire gates: %w", err)
	}
	// we pass the result in a goroutine until the wizard compiler is ready to receive it
	go func() {
		builder.rcCols <- cols
	}()
	return builder.storeCommitBuilder.Compile()
}

// Compiler returns the compiler of the underlying builder.
func (builder *externalRangeChecker) Compiler() frontend.Compiler {
	return builder.storeCommitBuilder.Compiler()
}

// addRangeCheckConstraints adds the wizard constraints implementing the range-checks
// requested by the gnark circuit.
func (ctx *compilationCtx) addRangeCheckConstraint() {

	var (
		round                            = ctx.Columns.L[0].Round()
		rcL                              = ctx.RangeCheckOption.RcL
		rcR                              = ctx.RangeCheckOption.RcR
		rcO                              = ctx.RangeCheckOption.RcO
		rcLValue                         = ctx.comp.Precomputed.MustGet(rcL.GetColID())
		rcRValue                         = ctx.comp.Precomputed.MustGet(rcR.GetColID())
		rcOValue                         = ctx.comp.Precomputed.MustGet(rcO.GetColID())
		numRcL                           = smartvectors.Sum(rcLValue)
		numRcR                           = smartvectors.Sum(rcRValue)
		numRcO                           = smartvectors.Sum(rcOValue)
		totalNumRangeCheckedValues       = numRcL.Uint64() + numRcR.Uint64() + numRcO.Uint64()
		totalNumRangeCheckedValuesPadded = utils.NextPowerOfTwo(totalNumRangeCheckedValues)
	)

	if totalNumRangeCheckedValues == 0 {
		// nothing to range-check. Note: we still declared rcL, rcR, rcO which
		// should be skipped also.
		ctx.RangeCheckOption.wasCancelled = true
		return
	}

	ctx.RangeCheckOption.RangeChecked = make([]ifaces.Column, len(ctx.Columns.L))
	ctx.RangeCheckOption.LimbDecomposition = make([]wizard.ProverAction, len(ctx.Columns.L))

	for i := range ctx.Columns.L {

		var (
			l            = ctx.Columns.L[i]
			r            = ctx.Columns.R[i]
			o            = ctx.Columns.O[i]
			rangeChecked = ctx.comp.InsertCommit(round, ctx.colIDf("RANGE_CHECKED_%v", i), utils.ToInt(totalNumRangeCheckedValuesPadded))
		)

		ctx.RangeCheckOption.RangeChecked[i] = rangeChecked

		ctx.comp.GenericFragmentedConditionalInclusion(
			round,
			ctx.queryIDf("RANGE_CHECKED_SELECTION_L_%v", i),
			[][]ifaces.Column{{rangeChecked}},
			[]ifaces.Column{l},
			nil,
			rcL,
		)

		ctx.comp.GenericFragmentedConditionalInclusion(
			round,
			ctx.queryIDf("RANGE_CHECKED_SELECTION_R_%v", i),
			[][]ifaces.Column{{rangeChecked}},
			[]ifaces.Column{r},
			nil,
			rcR,
		)

		ctx.comp.GenericFragmentedConditionalInclusion(
			round,
			ctx.queryIDf("RANGE_CHECKED_SELECTION_O_%v", i),
			[][]ifaces.Column{{rangeChecked}},
			[]ifaces.Column{o},
			nil,
			rcO,
		)

		_, ctx.RangeCheckOption.LimbDecomposition[i] = byte32cmp.Decompose(
			ctx.comp,
			rangeChecked,
			ctx.RangeCheckOption.NbLimbs,
			ctx.RangeCheckOption.NbBits,
		)
	}
}

func (ctx *GenericPlonkProverAction) assignRangeChecked(run *wizard.ProverRuntime) {

	var (
		rcL      = ctx.RangeCheckOption.RcL
		rcR      = ctx.RangeCheckOption.RcR
		rcO      = ctx.RangeCheckOption.RcO
		rcLValue = run.Spec.Precomputed.MustGet(rcL.GetColID()).IntoRegVecSaveAlloc()
		rcRValue = run.Spec.Precomputed.MustGet(rcR.GetColID()).IntoRegVecSaveAlloc()
		rcOValue = run.Spec.Precomputed.MustGet(rcO.GetColID()).IntoRegVecSaveAlloc()
	)

	parallel.Execute(len(ctx.RangeCheckOption.RangeChecked), func(start, stop int) {
		for i := start; i < stop; i++ {

			var (
				activated = ctx.Columns.Activators[i].GetColAssignment(run).Get(0)
				l         = ctx.Columns.L[i].GetColAssignment(run)
				r         = ctx.Columns.R[i].GetColAssignment(run)
				o         = ctx.Columns.O[i].GetColAssignment(run)
				rcSize    = ctx.RangeCheckOption.RangeChecked[i].Size()
				rc        = make([]field.Element, 0, rcSize)
			)

			if activated.IsZero() {
				run.AssignColumn(
					ctx.RangeCheckOption.RangeChecked[i].GetColID(),
					smartvectors.NewConstant(field.Zero(), rcSize),
				)
			} else {

				for i := range rcLValue {
					if rcLValue[i].IsOne() {
						rc = append(rc, l.Get(i))
					}

					if rcRValue[i].IsOne() {
						rc = append(rc, r.Get(i))
					}

					if rcOValue[i].IsOne() {
						rc = append(rc, o.Get(i))
					}
				}

				run.AssignColumn(
					ctx.RangeCheckOption.RangeChecked[i].GetColID(),
					smartvectors.RightZeroPadded(rc, rcSize),
				)
			}

			ctx.RangeCheckOption.LimbDecomposition[i].Run(run)
		}
	})
}

// Returns true if v is a constant in bound, panics if it is a constant but not
// in bound. Return false if not a constant.
func checkIfConst(v frontend.Variable, bits int) (isConst bool) {

	switch vv := v.(type) {
	default:
		return false
	case int:
		checkConstInt64(int64(vv), bits)
	case int8:
		checkConstInt64(int64(vv), bits)
	case int16:
		checkConstInt64(int64(vv), bits)
	case int32:
		checkConstInt64(int64(vv), bits)
	case int64:
		checkConstInt64(int64(vv), bits)
	case uint:
		checkConstUint64(uint64(vv), bits)
	case uint8:
		checkConstUint64(uint64(vv), bits)
	case uint16:
		checkConstUint64(uint64(vv), bits)
	case uint32:
		checkConstUint64(uint64(vv), bits)
	case uint64:
		checkConstUint64(uint64(vv), bits)
	case *big.Int:
		if vv.BitLen() > bits {
			utils.Panic("OOB constant: %v has more than %v bits", vv.String(), bits)
		}
	case field.Element:
		if vv.BitLen() > bits {
			utils.Panic("OOB constant: %v has more than %v bits", vv.String(), bits)
		}
	}

	return true
}

func checkConstInt64(vv int64, bits int) {
	if vv>>bits > 0 {
		utils.Panic("range-check on OOB constant: %v does not fit on %v bits", vv, bits)
	}
}

func checkConstUint64(vv uint64, bits int) {
	if vv>>bits > 0 {
		utils.Panic("range-check on OOB constant: %v does not fit on %v bits", vv, bits)
	}
}
