package globalcs

import (
	"math/big"
	"runtime"
	"sort"
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/arena"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
	ppool "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel/pool"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/profiling"
)

const (
	/*
		Explanation for Manual Garbage Collection Thresholds
	*/
	// These two thresholds work well for the real-world traces at the moment of writing and a 340GiB memory limit,
	// but this approach can be generalized and further improved.

	// When ctx.domainSize>=524288, proverEvaluationQueries() experiences a heavy workload,
	// consistently hitting the GOMEMLIMIT of 340GiB.
	// This results in numerous auto GCs during CPU-intensive small tasks, significantly degrading performance.
	// In the benchmark input files, GC_DOMAIN_SIZE >= 524288 means only the first call of proverEvaluationQueries().
	// With ctx.domainSize<=262144, manual GC is not necessary as auto GCs triggered by GOMEMLIMIT suffice.
	GC_DOMAIN_SIZE int = 524288
)

// QuotientCtx collects all the internal fields needed to compute the quotient
type QuotientCtx struct {

	// DomainSize is the domain over which the global constraints are computed
	DomainSize int

	// Ratio lists the ratio found in the global constraints
	//
	// See [mergingCtx.Ratios]
	Ratios []int

	// RootsPerRatio[k] stores all the root columns involved in the aggregate
	// expressions for ration Ratios[k]. By root column we mean the underlying
	// column that are actually committed to. For instance, if Shift(A, 1) is
	// in ColumnsForRatio[k], we will have A in RootPerRatio[k]
	RootsForRatio [][]ifaces.Column

	// AllInvolvedColumns stores the union of the ColumnForRatio[k] for all k
	AllInvolvedColumns []ifaces.Column

	// AggregateExpressions[k] stores the aggregate expression for Ratios[k]
	AggregateExpressions []*symbolic.Expression

	// QuotientShares[k] stores for each k, the list of the Ratios[k] shares
	// of the quotient for the AggregateExpression[k]
	QuotientShares [][]ifaces.Column
}

// createQuotientCtx constructs a [quotientCtx] from a list of ratios and aggregated
// expressions. The function organizes the handles but does not declare anything
// in the current wizard.CompiledIOP.
func createQuotientCtx(comp *wizard.CompiledIOP, ratios []int, aggregateExpressions []*symbolic.Expression) QuotientCtx {

	var (
		allInvolvedColumns = map[ifaces.ColID]struct{}{}
		_, _, domainSize   = wizardutils.AsExpr(aggregateExpressions[0])
		ctx                = QuotientCtx{
			DomainSize:           domainSize,
			Ratios:               ratios,
			AggregateExpressions: aggregateExpressions,
			AllInvolvedColumns:   []ifaces.Column{},
			RootsForRatio:        make([][]ifaces.Column, len(ratios)),
			QuotientShares:       generateQuotientShares(comp, ratios, domainSize),
		}
	)

	for k, expr := range ctx.AggregateExpressions {
		metadatas := expr.BoardListVariableMetadata()
		uniqueRootsForRatio := map[ifaces.ColID]struct{}{}

		// This loop scans the metadata looking for columns with the goal of
		// populating the collections composing quotientCtx.
		for _, metadata := range metadatas {

			// Scan in column metadata only
			col, ok := metadata.(ifaces.Column)
			if !ok {
				continue
			}

			// we keep track of all involved columns
			colName := col.GetColID()
			if _, ok := allInvolvedColumns[colName]; ok {
				continue
			}

			allInvolvedColumns[colName] = struct{}{}
			ctx.AllInvolvedColumns = append(ctx.AllInvolvedColumns, col)

			// first time we see this root for this ratio
			rootCol := column.RootParents(col)
			rootName := rootCol.GetColID()
			if _, ok := uniqueRootsForRatio[rootName]; !ok {
				uniqueRootsForRatio[rootName] = struct{}{}
				ctx.RootsForRatio[k] = append(ctx.RootsForRatio[k], rootCol)
			}

		}
	}

	// The above loop is supposedly iterating in deterministic order but we have
	// noticed some hard-to-find non-determinism in the compilation and this
	// cause problems in practice. So we sort the slices of the context to be
	// sure there is no issue.
	sortColumns := func(v []ifaces.Column) {
		sort.Slice(v, func(i, j int) bool {
			return v[i].GetColID() < v[j].GetColID()
		})
	}

	sortColumns(ctx.AllInvolvedColumns)

	for k := range ctx.RootsForRatio {
		sortColumns(ctx.RootsForRatio[k])
	}

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
			)
		}
	}

	return quotientShares
}

type computeQuotientCtx struct {
	rootsForRatio             [][]ifaces.Column
	aggregateExpressionsBoard []symbolic.ExpressionBoard
	maxExprNodes              int
	maxNbAllocs               int
	maxRatio                  int
}

// refineContext analyzes the context and the prover runtime to build a refined
// context that is more efficient to use during the actual quotient computation.
// In particular, it tries to simplify the expressions by doing constant
// propagation and also precomputes some statistics about the number of
// allocations needed.
func (ctx *QuotientCtx) refineContext(run *wizard.ProverRuntime) computeQuotientCtx {
	stopTimer := profiling.LogTimer("refine context for quotient computation")
	defer stopTimer()
	maxRatio := utils.Max(ctx.Ratios...)

	// let's simplify the boards if we can, by doing "constant propagation“
	// so we don't use the boards from the context, instead we build a
	// "translation map" to use with expresssion.Replay, reconstruct the expression
	// and build new boards.
	// idea: a significant number of variables may be constants; so we could end up with a simpler
	// board overall and allocates much less memory.

	// first we loop over all involved columns
	// if we identify a variable that is a constant, we replace its occurence in the symbolic expressions
	// by a symbolic.Constant
	translationMap := collection.NewMappingWithCapacity[string, *symbolic.Expression](len(ctx.RootsForRatio[0]))

	for j := range ctx.Ratios {
		roots := ctx.RootsForRatio[j]
		for _, col := range roots {
			name := col.GetColID()
			witness, isNatural := run.Columns.TryGet(name)
			if !isNatural {
				witness = col.GetColAssignment(run)
			}
			switch w := witness.(type) {
			case *sv.Constant:
				if _, ok := translationMap.TryGet(string(name)); ok {
					continue
				}
				translationMap.Update(string(name), symbolic.NewConstant(w.Value))
			}
		}

	}

	// for each ratios, we build in this function:
	rootsForRatio := make([][]ifaces.Column, len(ctx.Ratios))
	aggregateExpressionsBoard := make([]symbolic.ExpressionBoard, len(ctx.Ratios))

	// then for each aggregate expressions, we replay the expression (Variable -> Constant)
	parallel.Execute(len(ctx.AggregateExpressions), func(start, stop int) {
		for i := start; i < stop; i++ {
			expr := ctx.AggregateExpressions[i]
			e := expr.Replay(translationMap)
			board := e.Board()

			aggregateExpressionsBoard[i] = board

			uniqueRootsForRatio := map[ifaces.ColID]struct{}{}
			for _, metadata := range board.ListVariableMetadata() {
				col, ok := metadata.(ifaces.Column)
				if !ok {
					continue
				}

				// first time we see this root for this ratio
				rootCol := column.RootParents(col)
				rootName := rootCol.GetColID()
				if _, ok := uniqueRootsForRatio[rootName]; !ok {
					uniqueRootsForRatio[rootName] = struct{}{}
					rootsForRatio[i] = append(rootsForRatio[i], rootCol)
				}
			}
		}
	})

	// now we can compute some stats about the boards
	// we are interested in:
	// - maxExprNodes: the maximum number of nodes in any of the expression
	//   (after constant propagation)
	// - maxNbAllocs: the maximum number of allocations we may need to do
	//   when evaluating a quotient share. This is the sum, for each ratio
	//   of the number of non-constant roots involved in the expression.
	//   We multiply this by domainSize later when we create the arena
	//   for allocations.
	maxExprNodes := 0
	maxNbAllocs := 0
	for i := 0; i < maxRatio; i++ {
		rComputed := make(map[ifaces.ColID]struct{})
		allocsThisRound := 0
		for j, ratio := range ctx.Ratios {
			if i%(maxRatio/ratio) != 0 {
				continue
			}
			board := aggregateExpressionsBoard[j]
			nbNodesNew := board.CountNodesFilterConstants()

			roots := rootsForRatio[j]
			for _, root := range roots {
				name := root.GetColID()
				if _, found := rComputed[name]; found {
					continue
				}

				rComputed[name] = struct{}{}

				v, isNatural := run.Columns.TryGet(name)
				if !isNatural {
					v = root.GetColAssignment(run)
				}
				if _, ok := v.(*sv.Constant); !ok {
					allocsThisRound++
					nbNodesNew--
				}
			}
			maxExprNodes = max(maxExprNodes, nbNodesNew)
		}
		maxNbAllocs = max(maxNbAllocs, allocsThisRound)
	}

	cctx := computeQuotientCtx{
		maxRatio:                  maxRatio,
		rootsForRatio:             rootsForRatio,
		aggregateExpressionsBoard: aggregateExpressionsBoard,
		maxExprNodes:              maxExprNodes,
		maxNbAllocs:               maxNbAllocs,
	}
	return cctx
}

// Run implements the [wizard.ProverAction] interface and embeds the logic to
// compute the quotient shares.
func (ctx *QuotientCtx) Run(run *wizard.ProverRuntime) {
	stopTimer := profiling.LogTimer("quotient compute")
	defer stopTimer()

	if ctx.DomainSize >= GC_DOMAIN_SIZE {
		runtime.GC()
	}

	var (
		cctx           = ctx.refineContext(run)
		domain         = fft.NewDomain(uint64(ctx.DomainSize), fft.WithCache())
		vArena         = arena.NewVectorArena[field.Element](cctx.maxNbAllocs * ctx.DomainSize)
		vArenaEvaluate = arena.NewVectorArena[field.Element]((cctx.maxExprNodes * symbolic.ChunkSize()) * runtime.GOMAXPROCS(0))
	)

	// Precompute annulator inverses for all cosets
	chAnnulator := make(chan struct{}, 1)
	var annulatorInv []field.Element
	go func() {
		annulatorInv = fastpoly.EvalXnMinusOneOnACoset(ctx.DomainSize, ctx.DomainSize*cctx.maxRatio)
		annulatorInv = field.BatchInvert(annulatorInv)
		close(chAnnulator)
	}()

	var computedReeval sync.Map
	var wgAssignments sync.WaitGroup

	for i := 0; i < cctx.maxRatio; i++ {
		computedReeval.Clear()
		vArena.Reset(0)

		for j, ratio := range ctx.Ratios {
			if i%(cctx.maxRatio/ratio) != 0 {
				continue
			}

			share := i * ratio / cctx.maxRatio
			roots := cctx.rootsForRatio[j]
			board := cctx.aggregateExpressionsBoard[j]
			metadatas := board.ListVariableMetadata()

			shift := computeShift(uint64(ctx.DomainSize), ratio, share)
			domainCoset := fft.NewDomain(uint64(ctx.DomainSize), fft.WithCache(), fft.WithShift(shift))

			// Reevaluate roots on coset in parallel
			ppool.ExecutePoolChunky(len(roots), func(k int) {
				root := roots[k]
				name := root.GetColID()

				_v, found := computedReeval.Load(name)
				if found && _v != nil {
					return
				}
				// Mark as in-progress, this should be useless since we use "unique roots for ratio"
				// there shouldn't be any collisions
				computedReeval.Store(name, nil)

				v, isNatural := run.Columns.TryGet(name)
				if !isNatural {
					v = root.GetColAssignment(run)
				}
				reevaledRoot := reevalOnCoset(v, vArena, domain, domainCoset)
				computedReeval.Store(name, reevaledRoot)
			})

			// Prepare evaluation inputs for the constraint expression
			var wg sync.WaitGroup
			evalInputs := make([]sv.SmartVector, len(metadatas))
			for k := 0; k < len(metadatas); k++ {
				switch metadata := metadatas[k].(type) {
				case ifaces.Column:
					root := column.RootParents(metadata)
					rootName := root.GetColID()
					_reevaledRoot, _ := computedReeval.Load(rootName)
					reevaledRoot := _reevaledRoot.(sv.SmartVector)
					if !metadata.IsComposite() {
						evalInputs[k] = reevaledRoot
						continue
					}
					if shifted, ok := metadata.(column.Shifted); ok {
						evalInputs[k] = sv.SoftRotate(reevaledRoot, shifted.Offset)
						continue
					}
					utils.Panic("unexpected composite column type %T", metadata)
				case coin.Info:
					evalInputs[k] = sv.NewConstant(run.GetRandomCoinField(metadata.Name), ctx.DomainSize)
				case variables.X:
					wg.Add(1)
					go func(k, i int) {
						evalInputs[k] = metadata.EvalCoset(ctx.DomainSize, i, cctx.maxRatio, true)
						wg.Done()
					}(k, i)

				case variables.PeriodicSample:
					wg.Add(1)
					go func(k, i int) {
						evalInputs[k] = metadata.EvalCoset(ctx.DomainSize, i, cctx.maxRatio, true)
						wg.Done()
					}(k, i)
				case ifaces.Accessor:
					evalInputs[k] = sv.NewConstant(metadata.GetVal(run), ctx.DomainSize)
				default:
					utils.Panic("not a variable type %T", metadata)
				}
			}
			wg.Wait()

			// Evaluate and assign quotient share
			vArenaEvaluate.Reset(0)
			quotientShare := board.Evaluate(evalInputs, vArenaEvaluate)

			<-chAnnulator
			quotientShare = sv.ScalarMul(quotientShare, annulatorInv[i])
			run.AssignColumn(ctx.QuotientShares[j][share].GetColID(), quotientShare)
		}

	}

	vArena = nil
	vArenaEvaluate = nil
	computedReeval.Clear()

	wgAssignments.Wait()

	if ctx.DomainSize >= GC_DOMAIN_SIZE {
		runtime.GC()
	}

}

// reevalOnCoset takes a vector v in evaluation form on the base domain
// and returns the vector evaluated on the coset defined by (cosetRatio, cosetID)
func reevalOnCoset(v sv.SmartVector, vArena *arena.VectorArena, domain, domainCoset *fft.Domain) sv.SmartVector {
	skipInverse := false
	switch x := v.(type) {
	case *sv.Constant:
		return x
	case *sv.PaddedCircularWindow:
		interval := x.Interval()
		if interval.IntervalLen == 1 && interval.Start() == 0 && x.PaddingVal_.IsZero() {
			// It's a multiple of the first Lagrange polynomial c * (1 + x + x^2 + x^3 + ...)
			// The ifft is (c) = (c/N, c/N, c/N, ...)
			constTerm := field.NewElement(uint64(x.Len()))
			constTerm.Inverse(&constTerm)
			constTerm.Mul(&constTerm, &x.Window_[0])
			v = sv.NewConstant(constTerm, x.Len())
			skipInverse = true
		}
	}
	res := arena.Get[field.Element](vArena, v.Len())
	v.WriteInSlice(res)

	if !skipInverse {
		domain.FFTInverse(res, fft.DIF, fft.WithNbTasks(2))
	}

	domainCoset.FFT(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(2))
	return sv.NewRegular(res)
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
