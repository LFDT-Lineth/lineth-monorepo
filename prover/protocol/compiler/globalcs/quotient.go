package globalcs

import (
	"math/big"
	"reflect"
	"runtime"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/common/fastpoly"
	"github.com/consensys/linea-monorepo/prover/maths/common/fastpolyext"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/arena"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
)

// QuotientCtx collects all the internal fields needed to compute the quotient
type QuotientCtx struct {

	// DomainSize is the domain over which the global constraints are computed
	DomainSize int

	// Ratio lists the ratio found in the global constraints
	//
	// See [mergingCtx.Ratios]
	Ratios []int

	// ShiftedColumnsForRatio[k] stores all the columns involved in the aggregate
	// expressions for ratio Ratios[k]
	ShiftedColumnsForRatio [][]ifaces.Column

	// RootsPerRatio[k] stores all the root columns involved in the aggregate
	// expressions for ration Ratios[k]. By root column we mean the underlying
	// column that are actually committed to. For instance, if Shift(A, 1) is
	// in ColumnsForRatio[k], we will have A in RootPerRatio[k]
	RootsForRatio [][]ifaces.Column

	// AllInvolvedColumns stores the union of the ColumnForRatio[k] for all k
	AllInvolvedColumns []ifaces.Column

	// AllInvolvedRoots stores the union of the RootsForRatio[k] for all k
	AllInvolvedRoots []ifaces.Column

	// AggregateExpressions[k] stores the aggregate expression for Ratios[k]
	AggregateExpressions []*symbolic.Expression

	// AggregateExpressionsBoard[k] stores the topological sorting of
	// AggregateExpressions[k]
	AggregateExpressionsBoard []symbolic.ExpressionBoard

	// QuotientShares[k] stores for each k, the list of the Ratios[k] shares
	// of the quotient for the AggregateExpression[k]
	QuotientShares [][]ifaces.Column

	// ConstraintsByRatio[r] stores the list of indices k such that Ratios[k] == r
	ConstraintsByRatio map[int][]int
}

// createQuotientCtx constructs a [quotientCtx] from a list of ratios and aggregated
// expressions. The function organizes the handles but does not declare anything
// in the current wizard.CompiledIOP.
func createQuotientCtx(comp *wizard.CompiledIOP, ratios []int, aggregateExpressions []*symbolic.Expression) QuotientCtx {

	var (
		allInvolvedHandlesIndex = map[ifaces.ColID]struct{}{}
		allInvolvedRootsSet     = collection.NewSet[ifaces.ColID]()
		_, _, domainSize        = wizardutils.AsExpr(aggregateExpressions[0])
		ctx                     = QuotientCtx{
			DomainSize:                domainSize,
			Ratios:                    ratios,
			AggregateExpressions:      aggregateExpressions,
			AggregateExpressionsBoard: make([]symbolic.ExpressionBoard, len(ratios)),
			AllInvolvedColumns:        []ifaces.Column{},
			AllInvolvedRoots:          []ifaces.Column{},
			ShiftedColumnsForRatio:    make([][]ifaces.Column, len(ratios)),
			RootsForRatio:             make([][]ifaces.Column, len(ratios)),
			QuotientShares:            generateQuotientShares(comp, ratios, domainSize),
			ConstraintsByRatio:        make(map[int][]int),
		}
	)

	for k, ratio := range ratios {
		ctx.ConstraintsByRatio[ratio] = append(ctx.ConstraintsByRatio[ratio], k)
	}

	for k, expr := range ctx.AggregateExpressions {

		var (
			board               = expr.Board()
			uniqueRootsForRatio = collection.NewSet[ifaces.ColID]()
		)

		ctx.AggregateExpressionsBoard[k] = board

		// This loop scans the metadata looking for columns with the goal of
		// populating the collections composing quotientCtx.
		for _, metadata := range board.ListVariableMetadata() {

			// Scan in column metadata only
			col, ok := metadata.(ifaces.Column)
			if !ok {
				continue
			}

			var (
				rootCol = column.RootParents(col)
			)

			// Append the handle (we trust that there are no duplicate of handles
			// within a constraint). This works because the symbolic package has
			// automatic simplification routines that ensure that an expression
			// does not refer to duplicates of the same variable.
			if _, isShifted := col.(column.Shifted); isShifted {
				// TODO @gbotrel confirm we only get shifted and natural columns.
				ctx.ShiftedColumnsForRatio[k] = append(ctx.ShiftedColumnsForRatio[k], col)
			}

			if !uniqueRootsForRatio.Exists(rootCol.GetColID()) {
				ctx.RootsForRatio[k] = append(ctx.RootsForRatio[k], rootCol)
				uniqueRootsForRatio.Insert(rootCol.GetColID())
			}

			// Get the name of the
			if _, alreadyThere := allInvolvedHandlesIndex[col.GetColID()]; alreadyThere {
				continue
			}

			allInvolvedHandlesIndex[col.GetColID()] = struct{}{}
			ctx.AllInvolvedColumns = append(ctx.AllInvolvedColumns, col)

			// If the handle is simply a shift or a natural columns tracks its root
			if !allInvolvedRootsSet.Exists(rootCol.GetColID()) {
				allInvolvedRootsSet.Insert(rootCol.GetColID())
				ctx.AllInvolvedRoots = append(ctx.AllInvolvedRoots, rootCol)
			}
		}
	}

	// TODO @gbotrel this context preparation should compute the exact memory needed for the arenas
	// in Run and prepare them here.

	return ctx
}

// generateQuotientShares declares and returns the quotient share columns
func generateQuotientShares(comp *wizard.CompiledIOP, ratios []int, domainSize int) [][]ifaces.Column {

	var (
		quotientShares = make([][]ifaces.Column, len(ratios))
		currRound      = comp.NumRounds() - 1
	)

	for i, ratio := range ratios {
		quotientShares[i] = make([]ifaces.Column, ratio)
		for k := range quotientShares[i] {
			quotientShares[i][k] = comp.InsertCommit(
				currRound,
				ifaces.ColID(deriveName(comp, QUOTIENT_POLY_TMPL, ratio, k)),
				domainSize,
				false,
			)
		}
	}

	return quotientShares
}

// coeffEntry caches the iFFT (coefficient form) of a root column's witness.
// Computed once per ratio group and reused across all cosets.
type coeffEntry struct {
	isConst  bool
	isBase   bool
	constVal field.Element
	constExt fext.Element
	base     []field.Element
	ext      []fext.Element
}

// compute the quotient shares.
func (ctx *QuotientCtx) Run(run *wizard.ProverRuntime) {
	stopTimer := profiling.LogTimer("computed the quotient (domain size %d)", ctx.DomainSize)
	defer stopTimer()

	domain0 := fft.NewDomain(uint64(ctx.DomainSize), fft.WithCache())

	// Take the max quotient degree
	maxRatio := utils.Max(ctx.Ratios...)

	/*
		For the quotient, we precompute the values of (wQ^N - 1)^-1 for w in H, the
		larger domain.

		Those values are D-periodic, thus we only compute a single period.
		(Where D is the ratio of the sizes of the larger and the smaller domain)

		The first value is ignored because it correspond to the case where w^N = 1
		(i.e. w is in the smaller subgroup)
	*/
	var onceAnnulatorExt, onceAnnulatorBase sync.Once
	var annulatorInvValsExt []fext.Element
	var annulatorInvVals []field.Element

	// The arena only holds one ratio group's roots at a time (Reset between groups),
	// so size it for the largest group rather than all roots.
	maxGroupRoots := 0
	for _, constraintsIndices := range ctx.ConstraintsByRatio {
		seen := make(map[ifaces.ColID]struct{})
		for _, j := range constraintsIndices {
			for _, root := range ctx.RootsForRatio[j] {
				seen[root.GetColID()] = struct{}{}
			}
		}
		if len(seen) > maxGroupRoots {
			maxGroupRoots = len(seen)
		}
	}
	tArena := time.Now()
	arenaExt := arena.NewVectorArenaMmap[fext.Element](ctx.DomainSize * maxGroupRoots)
	defer arenaExt.Free()
	log.Infof("[quotient d=%d] arena alloc: maxGroupRoots=%d, size=%dGB, took=%v",
		ctx.DomainSize, maxGroupRoots, int64(ctx.DomainSize)*int64(maxGroupRoots)*16/1e9, time.Since(tArena))

	// Collect ALL unique roots across ALL ratio groups and compute their iFFT
	// coefficient forms once. This avoids redundant iFFT work when roots appear
	// in multiple ratio groups.
	globalRootsMap := make(map[ifaces.ColID]int) // ColID -> index in globalRoots
	var globalRoots []ifaces.Column
	for _, constraintsIndices := range ctx.ConstraintsByRatio {
		for _, j := range constraintsIndices {
			for _, root := range ctx.RootsForRatio[j] {
				id := root.GetColID()
				if _, ok := globalRootsMap[id]; !ok {
					globalRootsMap[id] = len(globalRoots)
					globalRoots = append(globalRoots, root)
				}
			}
		}
	}

	// Compute iFFT coefficient forms for all unique roots globally.
	// Adaptive FFT task count: when few roots exist, give each FFT more threads.
	tIFFTGlobal := time.Now()
	numNonConst := len(globalRoots) // upper bound; refined below
	globalCoeffCache := make([]coeffEntry, len(globalRoots))
	nbIFFTTasks := max(2, min(64, runtime.GOMAXPROCS(0)/max(1, numNonConst)))

	parallel.Execute(len(globalRoots), func(start, stop int) {
		for k := start; k < stop; k++ {
			root := globalRoots[k]
			rootName := root.GetColID()

			var witness sv.SmartVector
			witness, isAssigned := run.Columns.TryGet(rootName)
			if !isAssigned {
				witness = root.GetColAssignment(run)
			}

			// Skip FFTs for constant witnesses: the coset evaluation
			// of a constant polynomial is the constant itself.
			switch w := witness.(type) {
			case *sv.Constant:
				globalCoeffCache[k] = coeffEntry{isConst: true, isBase: true, constVal: w.Value}
				continue
			case *sv.ConstantExt:
				globalCoeffCache[k] = coeffEntry{isConst: true, isBase: false, constExt: w.Value}
				continue
			}

			if smartvectors.IsBase(witness) {
				coeffs := make([]field.Element, ctx.DomainSize)
				witness.WriteInSlice(coeffs)
				domain0.FFTInverse(coeffs, fft.DIF, fft.WithNbTasks(nbIFFTTasks))
				globalCoeffCache[k] = coeffEntry{isBase: true, base: coeffs}
			} else {
				coeffs := make([]fext.Element, ctx.DomainSize)
				witness.WriteInSliceExt(coeffs)
				domain0.FFTInverseExt(coeffs, fft.DIF, fft.WithNbTasks(nbIFFTTasks))
				globalCoeffCache[k] = coeffEntry{isBase: false, ext: coeffs}
			}
		}
	})
	log.Infof("[quotient d=%d] global iFFT: roots=%d, nbTasks=%d, took=%v",
		ctx.DomainSize, len(globalRoots), nbIFFTTasks, time.Since(tIFFTGlobal))

	// Compile all boards upfront (pointer receiver persists the result).
	for _, constraintsIndices := range ctx.ConstraintsByRatio {
		for _, j := range constraintsIndices {
			ctx.AggregateExpressionsBoard[j].Compile()
		}
	}

	// Outer loop: iterate by ratio group.
	for ratio, constraintsIndices := range ctx.ConstraintsByRatio {

		step := maxRatio / ratio

		// Identify unique roots for this ratio group (for coset FFT and logging).
		uniqueRootsMap := make(map[ifaces.ColID]int)
		var uniqueRoots []ifaces.Column
		for _, j := range constraintsIndices {
			for _, root := range ctx.RootsForRatio[j] {
				id := root.GetColID()
				if _, ok := uniqueRootsMap[id]; !ok {
					uniqueRootsMap[id] = len(uniqueRoots)
					uniqueRoots = append(uniqueRoots, root)
				}
			}
		}

		// Log bytecode statistics for the boards in this ratio group.
		for _, j := range constraintsIndices {
			board := &ctx.AggregateExpressionsBoard[j]
			s := board.BytecodeStats()
			log.Infof("[quotient d=%d] ratio=%d board[%d]: nodes=%d slots=%d ops: const=%d input=%d mul=%d lincomb=%d polyeval=%d",
				ctx.DomainSize, ratio, j, len(board.Nodes), board.NumSlots, s.Const, s.Input, s.Mul, s.LinComb, s.PolyEval)
		}
		// Count non-constant roots for adaptive FFT parallelism.
		numNonConstRoots := 0
		for _, root := range uniqueRoots {
			entry := &globalCoeffCache[globalRootsMap[root.GetColID()]]
			if !entry.isConst {
				numNonConstRoots++
			}
		}
		nbFFTTasks := max(2, min(64, runtime.GOMAXPROCS(0)/max(1, numNonConstRoots)))

		log.Infof("[quotient d=%d] ratio=%d, uniqueRoots=%d (nonConst=%d), constraints=%d, nbFFTTasks=%d",
			ctx.DomainSize, ratio, len(uniqueRoots), numNonConstRoots, len(constraintsIndices), nbFFTTasks)

		// 3. Inner loop: iterate over applicable cosets for this ratio group.
		var totalFFT, totalEval time.Duration
		for shareIdx := 0; shareIdx < ratio; shareIdx++ {

			i := shareIdx * step // global coset index (same as original loop)

			// reset the scratch offset for this coset
			arenaExt.Reset(0)

			var (
				shift = computeShift(uint64(ctx.DomainSize), ratio, shareIdx)
			)

			rootResults := make([]sv.SmartVector, len(uniqueRoots))

			domain := fft.NewDomain(uint64(ctx.DomainSize), fft.WithShift(shift), fft.WithCache())

			// For each root: constants are returned directly,
			// non-constants copy cached coefficients and apply forward FFT.
			tFFT := time.Now()
			parallel.Execute(len(uniqueRoots), func(start, stop int) {
				for k := start; k < stop; k++ {
					entry := &globalCoeffCache[globalRootsMap[uniqueRoots[k].GetColID()]]
					if entry.isConst {
						if entry.isBase {
							rootResults[k] = sv.NewConstant(entry.constVal, ctx.DomainSize)
						} else {
							rootResults[k] = sv.NewConstantExt(entry.constExt, ctx.DomainSize)
						}
						continue
					}
					if entry.isBase {
						res := arena.Get[field.Element](arenaExt, ctx.DomainSize)
						copy(res, entry.base)
						domain.FFT(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(nbFFTTasks))
						rootResults[k] = smartvectors.NewRegular(res)
					} else {
						res := arena.Get[fext.Element](arenaExt, ctx.DomainSize)
						copy(res, entry.ext)
						domain.FFTExt(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(nbFFTTasks))
						rootResults[k] = smartvectors.NewRegularExt(res)
					}
				}
			})
			totalFFT += time.Since(tFFT)

			computedReeval := make(map[ifaces.ColID]sv.SmartVector, len(uniqueRoots))
			for k, root := range uniqueRoots {
				computedReeval[root.GetColID()] = rootResults[k]
			}

			tEval := time.Now()
			for _, j := range constraintsIndices {
				var (
					handles   = ctx.ShiftedColumnsForRatio[j]
					board     = &ctx.AggregateExpressionsBoard[j]
					metadatas = board.ListVariableMetadata()
				)

				for _, pol := range handles {
					polName := pol.GetColID()
					if _, ok := computedReeval[polName]; ok {
						continue
					}

					root := column.RootParents(pol)
					rootName := root.GetColID()

					reevaledRoot := computedReeval[rootName]

					if shifted, isShifted := pol.(column.Shifted); isShifted {
						var res sv.SmartVector
						switch ssv := reevaledRoot.(type) {
						case *sv.Regular:
							res = sv.SoftRotate(ssv, shifted.Offset)
						case *sv.RegularExt:
							res = sv.SoftRotateExt(ssv, shifted.Offset)
						case *sv.Constant:
							res = ssv // rotation of a constant is a no-op
						case *sv.ConstantExt:
							res = ssv // rotation of a constant is a no-op
						}
						computedReeval[polName] = res
						continue
					}
					panic("never called")
				}

				// Evaluates the constraint expression on the coset
				evalInputs := make([]sv.SmartVector, len(metadatas))

				for k, metadataInterface := range metadatas {

					switch metadata := metadataInterface.(type) {
					case ifaces.Column:
						value, ok := computedReeval[metadata.GetColID()]
						if !ok {
							utils.Panic("did not find the reevaluation of %v", metadata.GetColID())
						}
						evalInputs[k] = value

					case coin.Info:
						if metadata.IsBase() {
							utils.Panic("unsupported, coins are always over field extensions")

						} else {
							evalInputs[k] = sv.NewConstantExt(run.GetRandomCoinFieldExt(metadata.Name), ctx.DomainSize)
						}
					case variables.X:
						evalInputs[k] = metadata.EvalCoset(ctx.DomainSize, i, maxRatio, true)
					case variables.PeriodicSample:
						evalInputs[k] = metadata.EvalCoset(ctx.DomainSize, i, maxRatio, true)
					case ifaces.Accessor:
						if metadata.IsBase() {
							evalInputs[k] = sv.NewConstant(metadata.GetVal(run), ctx.DomainSize)
						} else {
							evalInputs[k] = sv.NewConstantExt(metadata.GetValExt(run), ctx.DomainSize)
						}
					default:
						utils.Panic("Not a variable type %v", reflect.TypeOf(metadataInterface))
					}
				}

				// Note that this will panic if the expression contains "no commitment"
				// This should be caught already by the constructor of the constraint.
				quotientShare := board.Evaluate(evalInputs)
				switch qs := quotientShare.(type) {
				case *sv.Regular:
					onceAnnulatorBase.Do(func() {
						annulatorInvVals = fastpoly.EvalXnMinusOneOnACoset(ctx.DomainSize, ctx.DomainSize*maxRatio)
						annulatorInvVals = field.ParBatchInvert(annulatorInvVals, runtime.GOMAXPROCS(0))
					})

					vq := field.Vector(*qs)
					vq.ScalarMul(vq, &annulatorInvVals[i])
				case *sv.RegularExt:
					onceAnnulatorExt.Do(func() {
						annulatorInvValsExt = fastpolyext.EvalXnMinusOneOnACoset(ctx.DomainSize, ctx.DomainSize*maxRatio)
						annulatorInvValsExt = fext.ParBatchInvert(annulatorInvValsExt, runtime.GOMAXPROCS(0))
					})
					vq := extensions.Vector(*qs)
					vq.ScalarMul(vq, &annulatorInvValsExt[i])
				case *sv.Constant:
					onceAnnulatorBase.Do(func() {
						annulatorInvVals = fastpoly.EvalXnMinusOneOnACoset(ctx.DomainSize, ctx.DomainSize*maxRatio)
						annulatorInvVals = field.ParBatchInvert(annulatorInvVals, runtime.GOMAXPROCS(0))
					})
					var scaled field.Element
					scaled.Mul(&qs.Value, &annulatorInvVals[i])
					quotientShare = sv.NewConstant(scaled, ctx.DomainSize)
				case *sv.ConstantExt:
					onceAnnulatorExt.Do(func() {
						annulatorInvValsExt = fastpolyext.EvalXnMinusOneOnACoset(ctx.DomainSize, ctx.DomainSize*maxRatio)
						annulatorInvValsExt = fext.ParBatchInvert(annulatorInvValsExt, runtime.GOMAXPROCS(0))
					})
					var scaled fext.Element
					scaled.Mul(&qs.Value, &annulatorInvValsExt[i])
					quotientShare = sv.NewConstantExt(scaled, ctx.DomainSize)
				default:
					utils.Panic("unexpected type %T", quotientShare)
				}

				run.AssignColumn(ctx.QuotientShares[j][shareIdx].GetColID(), quotientShare)
			}
			totalEval += time.Since(tEval)
		}
		log.Infof("[quotient d=%d] ratio=%d cosets=%d totalFFT=%v totalEval=%v",
			ctx.DomainSize, ratio, ratio, totalFFT, totalEval)

		// Arena is reset per coset, no per-group cleanup needed.
	}

	// Free the global coefficient cache so GC can reclaim.
	globalCoeffCache = nil
}

func computeShift(n uint64, cosetRatio int, cosetID int) field.Element {
	var shift field.Element
	cardinality := ecc.NextPowerOfTwo(uint64(n))
	frMulGen := fft.GeneratorFullMultiplicativeGroup()
	omega, _ := fft.Generator(cardinality * uint64(cosetRatio))
	omega.Exp(omega, big.NewInt(int64(cosetID)))
	shift.Mul(&frMulGen, &omega)
	return shift
}
