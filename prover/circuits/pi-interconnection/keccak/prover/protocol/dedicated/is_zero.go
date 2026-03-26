package dedicated

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

type IsZeroCtx struct {
	CtxID     int
	Round     int
	C         *sym.Expression
	Mask      *sym.Expression
	InvOrZero ifaces.Column
	IsZero    ifaces.Column
	// PaddingVal is an optional value to be used to pad the column
	// in case the provided expression contains offsets.
	PaddingVal *field.Element
}

// IsZero returns a fully constrained binary column z such that
// c[i] = 0 <=> z[i] = 1 and c[i] != 0  <=> z[i] = 0. `c` can be either a
// symbolic expression or a column. If the input expression contains a shift,
// then the resulting column `z` is computed and constrained assuming the shift
// wraps around.
//
// The function also returns a context object that can be invoked to perform the
// assignment of `z` and the intermediate internal columns. It has to be called
// explictly by the the caller during the prover runtime.
func IsZero(comp *wizard.CompiledIOP, c any) *IsZeroCtx {

	var (
		ctx = &IsZeroCtx{
			CtxID: len(comp.QueriesNoParams.AllKeys()),
		}
	)

	switch c1 := c.(type) {
	case ifaces.Column:
		ctx.Round = c1.Round()
		ctx.C = sym.NewVariable(c1)
	case *sym.Expression:
		ctx.C = c1
		ctx.Round = wizardutils.LastRoundToEval(c1)
	}

	compileIsZeroWithSize(comp, ctx)

	return ctx
}

// WithPaddingVal adds a padding value to the context
func (ctx *IsZeroCtx) WithPaddingVal(val field.Element) *IsZeroCtx {

	if !val.IsOne() && !val.IsZero() {
		utils.Panic("padding value must be 0 or 1, was %v", val.String())
	}

	ctx.PaddingVal = &val
	return ctx
}

// GetColumnAndProverAction returns the column and the prover action. This
// method is useful to diminish the change needed since we added [WithPaddingVal].
func (ctx *IsZeroCtx) GetColumnAndProverAction() (ifaces.Column, wizard.ProverAction) {
	return ctx.IsZero, ctx
}

// IsZeroMasked is an [IsZero] but allows passing an additional `mask`,
// expectedly a binary column. The caller is responsible for
// ensuring/constraining that the column is binary. All the generated columns
// will be zeroied when the mask is up.
func IsZeroMask(comp *wizard.CompiledIOP, c, mask any) *IsZeroCtx {

	var (
		ctx = &IsZeroCtx{
			CtxID: len(comp.QueriesNoParams.AllKeys()),
		}
	)

	var size int
	ctx.C, ctx.Round, size = wizardutils.AsExpr(c)
	m, roundMask, sizeMask := wizardutils.AsExpr(mask)
	ctx.Round = max(roundMask, ctx.Round)

	if sizeMask != size {
		utils.Panic("the size of the mask if %v but the column's size is %v", sizeMask, size)
	}

	ctx.Mask = m

	compileIsZeroWithSize(comp, ctx)

	return ctx
}

func compileIsZeroWithSize(comp *wizard.CompiledIOP, ctx *IsZeroCtx) {

	_, _, size := wizardutils.AsExpr(ctx.C)

	ctx.IsZero = comp.InsertCommit(
		ctx.Round,
		ifaces.ColIDf("IS_ZERO_%v_RES_%v", ctx.CtxID, ctx.Round),
		size,
	)

	ctx.InvOrZero = comp.InsertCommit(
		ctx.Round,
		ifaces.ColIDf("IS_ZERO_%v_INVERSE_OR_ZERO_%v", ctx.CtxID, ctx.Round),
		size,
	)

	var mask = any(1)
	if ctx.Mask != nil {
		mask = ctx.Mask
	}

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("IS_ZERO_%v_RES_IS_ONE_IF_C_ISZERO", ctx.CtxID),
		sym.Add(ctx.IsZero, sym.Mul(ctx.InvOrZero, ctx.C), sym.Neg(mask)),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("IS_ZERO_%v_RES_IS_ZERO_IF_C_ISNONZERO", ctx.CtxID),
		sym.Mul(ctx.IsZero, ctx.C),
	)

	if ctx.Mask != nil {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("IS_ZERO_%v_RES_IS_MASKED", ctx.CtxID),
			sym.Sub(ctx.IsZero, sym.Mul(mask, ctx.IsZero)),
		)
	}
}

// Run implements the [wizard.ProverAction] interface
func (ctx *IsZeroCtx) Run(run *wizard.ProverRuntime) {

	var (
		c                    = column.EvalExprColumn(run, ctx.C.Board()).IntoRegVecSaveAlloc()
		offsetRange          = query.MinMaxOffset(ctx.C)
		minOffset, maxOffset = offsetRange.Min, offsetRange.Max
	)

	// In case the expression contains offsets, the concrete value of
	// isZero won't be constrained. However, we still need to pay attention to
	// the value set in the "non-checked" positions because the limitless prover
	// might decide to extend the column by padding on the left or the right
	// using the initial or the final value. In that case, it is safer to use
	// the first/last non-ignored value.
	if minOffset < 0 {
		paddingVal := c[-minOffset]

		if ctx.PaddingVal != nil && ctx.PaddingVal.IsOne() {
			// When using a padding value of 1, we mean the isZero column should
			// be padded with 1s. To obtain this, we pad the base column with 0s
			// and [smartvectors.IsZero] will return 1s.
			paddingVal = field.Zero()
		}

		if ctx.PaddingVal != nil && ctx.PaddingVal.IsZero() {
			paddingVal = field.One()
		}

		for p := 0; p < -minOffset; p++ {
			c[p] = paddingVal
		}
	}

	if maxOffset > 0 {
		paddingVal := c[len(c)-maxOffset-1]

		if ctx.PaddingVal != nil && ctx.PaddingVal.IsOne() {
			// When using a padding value of 1, we mean the isZero column should
			// be padded with 1s. To obtain this, we pad the base column with 0s
			// and [smartvectors.IsZero] will return 1s.
			paddingVal = field.Zero()
		}

		if ctx.PaddingVal != nil && ctx.PaddingVal.IsZero() {
			paddingVal = field.One()
		}

		for p := len(c) - maxOffset; p < len(c); p++ {
			c[p] = paddingVal
		}
	}

	var (
		cSV       = smartvectors.NewRegular(c)
		invOrZero = smartvectors.BatchInvert(cSV)
		isZero    = smartvectors.IsZero(cSV)
	)

	if ctx.Mask != nil {

		mask := column.EvalExprColumn(run, ctx.Mask.Board())

		if mask.Len() != len(c) {
			valueOfC := ctx.C.MarshalJSONString()
			utils.Panic("the size of the mask if %v but the column's size is %v, mask=%v", mask.Len(), len(c), valueOfC)
		}

		if mask.Len() != invOrZero.Len() {
			utils.Panic("the size of the mask if %v but inv or zero's size is %v", mask.Len(), invOrZero.Len())
		}

		if mask.Len() != isZero.Len() {
			utils.Panic("the size of the mask if %v but the column's size is %v", mask.Len(), isZero.Len())
		}

		invOrZero = smartvectors.Mul(invOrZero, mask)
		isZero = smartvectors.Mul(isZero, mask)
	}

	run.AssignColumn(ctx.InvOrZero.GetColID(), invOrZero, wizard.DisableAssignmentSizeReduction)
	run.AssignColumn(ctx.IsZero.GetColID(), isZero, wizard.DisableAssignmentSizeReduction)
}
