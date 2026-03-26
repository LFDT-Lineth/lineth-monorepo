package mpts

// Option are options for the MultiPointToSinglePointCompilation
type Option func(*MultipointToSinglepointCompilation)

// WithNumColumnProfileOpt tells the compiler to add shadow columns (columns
// equal to zero everywhere to the comp). These columns are added so that
// the number of columns for each round matches the sizes indicated in the
// profile. The positions in the provided slices are understood as "starting
// from the first non-empty" rounds that are compiled by the current
// compilation context.
func WithNumColumnProfileOpt(numColProfileOpt []int, numColPrecomputed int) Option {
	return func(ctx *MultipointToSinglepointCompilation) {
		ctx.NumColumnProfileOpt = numColProfileOpt
		ctx.NumColumnProfilePrecomputed = numColPrecomputed
	}
}

// AddNonConstrainedColumns adds the non-constrained columns to the Polys and
// thus, include them in the Grail query. This is needed for the limitless
// prover because the GL module will contain columns that are initially only
// lookup-constrained (i.e. they are constrained in the LPP module but not in
// the GL module).
//
// When activated, the columns are added to the Grail query but the compiler
// does not do anything else with the evaluation points.
func AddUnconstrainedColumns() Option {
	return func(ctx *MultipointToSinglepointCompilation) {
		ctx.AddUnconstrainedColumnsOpt = true
	}
}
