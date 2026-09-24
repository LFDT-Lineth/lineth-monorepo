package fri

import (
	"fmt"
	"math/big"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark/frontend"
)

// This file is the in-circuit counterpart of pcs.Verify. Every function here
// mirrors a native one of the same name (minus the Gnark suffix) and must be
// kept in lock-step with it: the native verifier is the specification.
//
// Structural facts (proof geometry, layout, shapes, which Merkle levels carry
// an auxiliary row pair) are compile-time and are checked with panics.
// Value facts (roots, fold values, claims) become constraints. Query positions
// are circuit variables: every position-dependent selection is expressed on
// their bit decomposition, so the circuit shape does not depend on them.
//
// One deliberate divergence: the native verifier deduplicates input trees by
// root value. Roots are variables here, so batches are assumed to have
// pairwise distinct roots; a proof with a collision has fewer input openings
// per query than the circuit expects and is rejected as malformed.

// GnarkOpeningProof mirrors [OpeningProof].
type GnarkOpeningProof struct {
	InputQueries []GnarkInputQuery
	FRIProof     GnarkProof
}

// GnarkInputQuery mirrors [InputQuery].
type GnarkInputQuery []GnarkInputTreeOpening

// GnarkInputTreeOpening mirrors [InputTreeOpening]. A level without an
// auxiliary pair (a nil entry natively) is a zero-width [GnarkRowPair]; a
// present level always has at least one row, so the encoding is unambiguous.
type GnarkInputTreeOpening struct {
	Siblings []poseidon2.KoalagnarkOctuplet
	Leaves   []GnarkRowPair
}

// GnarkRowPair mirrors [RowPair].
type GnarkRowPair [2]GnarkRowOpening

// GnarkRowOpening mirrors [RowOpening].
type GnarkRowOpening struct {
	Base []circuit.Element
	Ext  []circuit.Ext
}

func (r GnarkRowOpening) isAbsent() bool { return len(r.Base) == 0 && len(r.Ext) == 0 }

// GnarkProof mirrors [Proof]. RunningQueries[k][j-1] is the single branch of
// folding round j for query k (running layers are backed by exactly one tree).
type GnarkProof struct {
	RoundRoots     []poseidon2.KoalagnarkOctuplet
	FinalPoly      []circuit.Ext
	RunningQueries []GnarkRunningQuery
}

// GnarkRunningQuery mirrors [RunningQuery] with the one-tree layer flattened.
type GnarkRunningQuery []GnarkBranch

// GnarkBranch mirrors [Branch] for the running trees, which carry no auxiliary
// siblings.
type GnarkBranch struct {
	Leaf     poseidon2.KoalagnarkOctuplet
	Siblings []poseidon2.KoalagnarkOctuplet
}

// GnarkBatchClaimedValues mirrors [BatchClaimedValues].
type GnarkBatchClaimedValues []GnarkSizedClaimedValues

// GnarkSizedClaimedValues mirrors [SizedClaimedValues].
type GnarkSizedClaimedValues struct {
	Base [][]circuit.Ext
	Ext  [][]circuit.Ext
}

// GnarkVerifyInputs mirrors [VerifyInputs]. Shapes and Shifts are structural
// and stay native; everything else is a circuit variable.
type GnarkVerifyInputs struct {
	Roots          []poseidon2.KoalagnarkOctuplet
	Shapes         []Shape
	Shifts         []BatchShifts
	ClaimedValues  []GnarkBatchClaimedValues
	Zeta           circuit.Ext
	FoldAlphas     []circuit.Ext
	QueryPositions []frontend.Variable
}

// =============================================================================
// Witness conversion
// =============================================================================

// NewGnarkOpeningProof converts a native opening proof into its witness
// assignment.
func NewGnarkOpeningProof(p OpeningProof) GnarkOpeningProof {
	return convertOpeningProof(p, true)
}

// AllocateGnarkOpeningProof returns an unassigned proof with the same geometry
// as p, for circuit compilation.
func AllocateGnarkOpeningProof(p OpeningProof) GnarkOpeningProof {
	return convertOpeningProof(p, false)
}

func convertOpeningProof(p OpeningProof, withValues bool) GnarkOpeningProof {
	res := GnarkOpeningProof{
		InputQueries: make([]GnarkInputQuery, len(p.InputQueries)),
		FRIProof: GnarkProof{
			RoundRoots:     make([]poseidon2.KoalagnarkOctuplet, len(p.FRIProof.RoundRoots)),
			FinalPoly:      make([]circuit.Ext, len(p.FRIProof.FinalPoly)),
			RunningQueries: make([]GnarkRunningQuery, len(p.FRIProof.RunningQueries)),
		},
	}
	for k, q := range p.InputQueries {
		res.InputQueries[k] = make(GnarkInputQuery, len(q))
		for i, branch := range q {
			res.InputQueries[k][i] = convertInputTreeOpening(branch, withValues)
		}
	}
	for i, root := range p.FRIProof.RoundRoots {
		res.FRIProof.RoundRoots[i] = convertOctuplet(root, withValues)
	}
	for i, c := range p.FRIProof.FinalPoly {
		res.FRIProof.FinalPoly[i] = convertExt(c, withValues)
	}
	for k, rq := range p.FRIProof.RunningQueries {
		res.FRIProof.RunningQueries[k] = make(GnarkRunningQuery, len(rq))
		for j, layer := range rq {
			if len(layer) != 1 {
				panic(fmt.Sprintf("fri: running layer must be backed by exactly one tree, got %d", len(layer)))
			}
			res.FRIProof.RunningQueries[k][j] = GnarkBranch{
				Leaf:     convertOctuplet(layer[0].Leaf, withValues),
				Siblings: convertOctuplets(layer[0].Siblings, withValues),
			}
		}
	}
	return res
}

func convertInputTreeOpening(b InputTreeOpening, withValues bool) GnarkInputTreeOpening {
	res := GnarkInputTreeOpening{
		Siblings: convertOctuplets(b.Siblings, withValues),
		Leaves:   make([]GnarkRowPair, len(b.Leaves)),
	}
	for i, pair := range b.Leaves {
		if pair == nil {
			continue
		}
		for s := range pair {
			res.Leaves[i][s] = convertRowOpening(pair[s], withValues)
		}
	}
	return res
}

func convertRowOpening(r RowOpening, withValues bool) GnarkRowOpening {
	res := GnarkRowOpening{
		Base: make([]circuit.Element, len(r.Base)),
		Ext:  make([]circuit.Ext, len(r.Ext)),
	}
	for i := range r.Base {
		if withValues {
			res.Base[i] = circuit.NewElementFromKoala(r.Base[i])
		}
	}
	for i := range r.Ext {
		res.Ext[i] = convertExt(r.Ext[i], withValues)
	}
	return res
}

func convertOctuplets(os []field.Octuplet, withValues bool) []poseidon2.KoalagnarkOctuplet {
	res := make([]poseidon2.KoalagnarkOctuplet, len(os))
	for i := range os {
		res[i] = convertOctuplet(os[i], withValues)
	}
	return res
}

func convertOctuplet(o field.Octuplet, withValues bool) poseidon2.KoalagnarkOctuplet {
	if !withValues {
		return poseidon2.KoalagnarkOctuplet{}
	}
	return poseidon2.NewKoalagnarkOctuplet(o)
}

func convertExt(e field.Ext, withValues bool) circuit.Ext {
	if !withValues {
		return circuit.Ext{}
	}
	return circuit.NewExt(e)
}

// =============================================================================
// Verify
// =============================================================================

// VerifyGnark is the in-circuit counterpart of [PCS.Verify]. Structural
// mismatches panic at circuit-definition time; every value check is a
// constraint.
func (pcs *PCS) VerifyGnark(api frontend.API, in GnarkVerifyInputs, proof GnarkOpeningProof) {
	if len(in.Roots) != len(in.Shapes) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: got %d roots, %d shapes", len(in.Roots), len(in.Shapes)))
	}
	layout, err := canonicalLayout(in.Shapes, in.Shifts)
	if err != nil {
		panic(err)
	}
	checkGnarkClaimShapes(in.ClaimedValues, in.Shifts)
	pcs, err = pcs.restrictTo(layout.maxSizeLog2())
	if err != nil {
		panic(err)
	}
	if len(proof.InputQueries) != int(pcs.Params.NumQueries) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: proof has %d input queries, want %d",
			len(proof.InputQueries), pcs.Params.NumQueries))
	}
	if len(in.QueryPositions) < int(pcs.Params.NumQueries) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: %d query positions, need at least %d",
			len(in.QueryPositions), pcs.Params.NumQueries))
	}
	checkGnarkOpeningProofShape(pcs.Params, proof.FRIProof, in.FoldAlphas)

	kapi := circuit.NewAPI(api)
	pcs.assertClaimPointsOutOfDomainGnark(kapi, layout, in.Zeta)

	orders := batchOrders(layout)
	inputRoots, inputIndexByBatch := inputOpeningRootsGnark(layout, orders, in.Roots)

	runningRoots := make([]poseidon2.KoalagnarkOctuplet, pcs.Params.numRounds())
	for j := uint8(1); j < pcs.Params.numRounds(); j++ {
		runningRoots[j] = proof.FRIProof.RoundRoots[j-1]
	}

	vq := gnarkVerifyQueryCtx{
		api:               kapi,
		pcs:               pcs,
		layout:            layout,
		proof:             proof,
		inputRoots:        inputRoots,
		runningRoots:      runningRoots,
		layoutClaims:      pcs.layoutClaimsGnark(kapi, layout, in.ClaimedValues, in.Zeta),
		inputIndexByBatch: inputIndexByBatch,
		orders:            orders,
		shapes:            in.Shapes,
		foldAlphas:        in.FoldAlphas,
		finalPoly:         proof.FRIProof.FinalPoly,
	}

	resolved := make([]gnarkResolvedQuery, pcs.Params.NumQueries)
	for k := range resolved {
		// ToBinary constrains the position into [0, 2^LogCodewordSize).
		posBits := api.ToBinary(in.QueryPositions[k], int(pcs.Params.LogCodewordSize))
		resolved[k] = vq.resolve(k, posBits)
	}
	checkFoldsGnark(kapi, pcs.Params, resolved, in.FoldAlphas)
}

func checkGnarkClaimShapes(claimed []GnarkBatchClaimedValues, shifts []BatchShifts) {
	if len(claimed) != len(shifts) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: got %d claimed batches, want %d", len(claimed), len(shifts)))
	}
	for b := range shifts {
		if len(claimed[b]) != len(shifts[b]) {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: batch %d has %d claimed sizes, want %d",
				b, len(claimed[b]), len(shifts[b])))
		}
		for s, sizedShifts := range shifts[b] {
			sized := claimed[b][s]
			if len(sized.Base) != len(sizedShifts.Base) || len(sized.Ext) != len(sizedShifts.Ext) {
				panic(fmt.Sprintf("fri: pcs.VerifyGnark: batch %d size %d claim rows mismatch", b, s))
			}
			for r := range sizedShifts.Base {
				if len(sized.Base[r]) != len(sizedShifts.Base[r]) {
					panic(fmt.Sprintf("fri: pcs.VerifyGnark: batch %d size %d base row %d claims mismatch", b, s, r))
				}
			}
			for r := range sizedShifts.Ext {
				if len(sized.Ext[r]) != len(sizedShifts.Ext[r]) {
					panic(fmt.Sprintf("fri: pcs.VerifyGnark: batch %d size %d ext row %d claims mismatch", b, s, r))
				}
			}
		}
	}
}

// checkGnarkOpeningProofShape mirrors [checkOpeningProofShape] minus the
// position range check, which ToBinary enforces on the variable positions.
func checkGnarkOpeningProofShape(p Params, prf GnarkProof, foldAlphas []circuit.Ext) {
	var wantRoundRoots uint8
	if p.numRounds() > 0 {
		wantRoundRoots = p.numRounds() - 1
	}
	if len(prf.RoundRoots) != int(wantRoundRoots) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: proof has %d round roots, want %d", len(prf.RoundRoots), wantRoundRoots))
	}
	if len(prf.RunningQueries) != int(p.NumQueries) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: proof has %d running queries, want %d",
			len(prf.RunningQueries), p.NumQueries))
	}
	if want := 1 << p.logFinalPolySize; len(prf.FinalPoly) != want {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: FinalPoly has %d entries, want %d", len(prf.FinalPoly), want))
	}
	if len(foldAlphas) < int(p.numRounds()) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: %d folding challenges, need at least %d", len(foldAlphas), p.numRounds()))
	}
	for k, q := range prf.RunningQueries {
		if len(q) != int(wantRoundRoots) {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: query %d has %d running layers, want %d", k, len(q), wantRoundRoots))
		}
	}
}

// assertClaimPointsOutOfDomainGnark mirrors [PCS.checkClaimPointsOutOfDomain].
// zeta lies in a codeword domain of cardinality c iff zeta^c == 1 (the roots
// of X^c - 1 are exactly that base-field subgroup), so the base-field test of
// the native version is not needed.
func (pcs *PCS) assertClaimPointsOutOfDomainGnark(api *circuit.KoalaBearAPI, layout layout, zeta circuit.Ext) {
	for _, bundle := range layout {
		if int(bundle.SizeLog2) >= len(pcs.Encoders) {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: size %d is outside params schedule", bundle.SizeLog2))
		}
		card := pcs.Encoders[bundle.SizeLog2].Domain.Cardinality
		pow := zeta
		for i := 0; i < bits.TrailingZeros64(card); i++ {
			pow = api.SquareExt(pow)
		}
		diff := api.SubExt(pow, api.OneExt())
		api.Frontend().AssertIsEqual(api.IsZeroExt(diff), 0)
	}
}

// inputOpeningRootsGnark mirrors [inputOpeningRoots] without deduplication:
// every batch gets its own input tree, in order of first appearance.
func inputOpeningRootsGnark(
	layout layout, orders [][]int, roots []poseidon2.KoalagnarkOctuplet,
) ([]poseidon2.KoalagnarkOctuplet, []int) {
	indexByBatch := make([]int, len(roots))
	for i := range indexByBatch {
		indexByBatch[i] = -1
	}
	inputRoots := make([]poseidon2.KoalagnarkOctuplet, 0, len(roots))
	for levelIdx := range layout {
		for _, batchIdx := range orders[levelIdx] {
			if indexByBatch[batchIdx] >= 0 {
				continue
			}
			indexByBatch[batchIdx] = len(inputRoots)
			inputRoots = append(inputRoots, roots[batchIdx])
		}
	}
	return inputRoots, indexByBatch
}

// gnarkQuotientClaim mirrors [quotientClaim].
type gnarkQuotientClaim struct {
	Point circuit.Ext
	Value circuit.Ext
}

// layoutClaimsGnark precomputes every entry's claim points and values, as the
// native Verify does before the query loop. Claim points are zeta scaled by a
// constant root of unity.
func (pcs *PCS) layoutClaimsGnark(
	api *circuit.KoalaBearAPI, layout layout, claimed []GnarkBatchClaimedValues, zeta circuit.Ext,
) [][][]gnarkQuotientClaim {
	res := make([][][]gnarkQuotientClaim, len(layout))
	for levelIdx, bundle := range layout {
		res[levelIdx] = make([][]gnarkQuotientClaim, len(bundle.Entries))
		for i, entry := range bundle.Entries {
			values := claimedRowGnark(claimed, entry)
			claims := make([]gnarkQuotientClaim, len(entry.Shifts))
			for s, shift := range entry.Shifts {
				var rotation field.Element
				rotation.Exp(pcs.Encoders[entry.SizeLog2].smallDomain.Generator, big.NewInt(int64(shift)))
				claims[s] = gnarkQuotientClaim{
					Point: api.MulByFpExt(zeta, api.ConstBig(rotation.BigInt(new(big.Int)))),
					Value: values[s],
				}
			}
			res[levelIdx][i] = claims
		}
	}
	return res
}

func claimedRowGnark(claimed []GnarkBatchClaimedValues, entry deepEntry) []circuit.Ext {
	sized := claimed[entry.BatchIdx][entry.SizeLog2]
	if entry.IsExt {
		return sized.Ext[entry.RowIdx]
	}
	return sized.Base[entry.RowIdx]
}

// gnarkInputPair mirrors [inputPair].
type gnarkInputPair struct {
	Self    circuit.Ext
	Sibling circuit.Ext
}

// gnarkResolvedQuery mirrors [resolvedQuery]; XInv is the inverse of the
// query's layer-0 domain point, needed by the fold recurrence.
type gnarkResolvedQuery struct {
	Rounds []gnarkInputPair
	Aux    map[uint8]gnarkInputPair
	Final  circuit.Ext
	XInv   circuit.Element
}

// gnarkVerifyQueryCtx mirrors [verifyQueryCtx].
type gnarkVerifyQueryCtx struct {
	api               *circuit.KoalaBearAPI
	pcs               *PCS
	layout            layout
	proof             GnarkOpeningProof
	inputRoots        []poseidon2.KoalagnarkOctuplet
	runningRoots      []poseidon2.KoalagnarkOctuplet
	layoutClaims      [][][]gnarkQuotientClaim
	inputIndexByBatch []int
	orders            [][]int
	shapes            []Shape
	foldAlphas        []circuit.Ext
	finalPoly         []circuit.Ext
}

// resolve mirrors [verifyQueryCtx.resolve] for one query whose position is
// given as little-endian bits over the layer-0 codeword domain.
func (vq gnarkVerifyQueryCtx) resolve(queryIdx int, posBits []frontend.Variable) gnarkResolvedQuery {
	pcs, api := vq.pcs, vq.api
	numRounds := pcs.Params.numRounds()

	finalDomain := pcs.Params.domainsLight[numRounds]
	xFinal := domainPointGnark(api, finalDomain, posBits[numRounds:])
	rq := gnarkResolvedQuery{
		Rounds: make([]gnarkInputPair, numRounds+1),
		Aux:    make(map[uint8]gnarkInputPair, len(vq.layout)),
		Final:  api.HornerExt(vq.finalPoly, xFinal),
		XInv:   domainPointInvGnark(api, pcs.Params.domainsLight[0], posBits),
	}
	zero := api.ZeroExt()
	for j := range rq.Rounds {
		rq.Rounds[j] = gnarkInputPair{Self: zero, Sibling: zero}
	}

	inputOpening := vq.proof.InputQueries[queryIdx]
	authenticateInputQueryGnark(api, pcs.Params, inputOpening, vq.inputRoots, posBits)

	for j := uint8(1); j < numRounds; j++ {
		branch := vq.proof.FRIProof.RunningQueries[queryIdx][j-1]
		if want := int(pcs.Params.LogCodewordSize - j); len(branch.Siblings) != want {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: query %d round %d: branch has %d siblings, want %d",
				queryIdx, j, len(branch.Siblings), want))
		}
		root := recoverRootGnark(api, branch, posBits[j:])
		assertOctupletEqual(api, root, vq.runningRoots[j])
		rq.Rounds[j] = gnarkInputPair{
			Self:    octupletToExtGnark(api, branch.Leaf),
			Sibling: octupletToExtGnark(api, branch.Siblings[len(branch.Siblings)-1]),
		}
	}

	for levelIdx, bundle := range vq.layout {
		round, err := pcs.roundForSize(bundle.SizeLog2)
		if err != nil {
			panic(err)
		}
		if round > numRounds {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: level %d introduced at round %d, must be <= %d",
				levelIdx, round, numRounds))
		}
		domain := pcs.Params.domainsLight[round]
		levelSize := int(domain.cardinality)
		bindInputTreeOpeningsGnark(inputOpening, vq.inputIndexByBatch, levelSize, vq.orders[levelIdx], bundle, vq.shapes)

		var alphaDeep circuit.Ext
		switch {
		case int(round) < len(vq.foldAlphas):
			alphaDeep = api.SquareExt(vq.foldAlphas[round])
		case len(vq.foldAlphas) > 0:
			alphaDeep = vq.foldAlphas[len(vq.foldAlphas)-1]
		default:
			alphaDeep = api.ZeroExt()
		}

		xSelf := domainPointGnark(api, domain, posBits[round:])
		xSib := api.NegExt(xSelf)
		entryClaims := vq.layoutClaims[levelIdx]
		rq.Aux[round] = gnarkInputPair{
			Self: reconstructQueryValueAtGnark(api, bundle, entryClaims, inputOpening, vq.inputIndexByBatch,
				levelSize, alphaDeep, xSelf, false, rq.Rounds[round].Self),
			Sibling: reconstructQueryValueAtGnark(api, bundle, entryClaims, inputOpening, vq.inputIndexByBatch,
				levelSize, alphaDeep, xSib, true, rq.Rounds[round].Sibling),
		}
	}

	if numRounds == 0 {
		pair := rq.Aux[0]
		api.AssertIsEqualExt(pair.Self, rq.Final)
		api.AssertIsEqualExt(pair.Sibling, api.HornerExt(vq.finalPoly, api.NegExt(xFinal)))
	}
	return rq
}

// checkFoldsGnark mirrors [checkFolds].
func checkFoldsGnark(api *circuit.KoalaBearAPI, p Params, resolved []gnarkResolvedQuery, foldAlphas []circuit.Ext) {
	halfBig := new(big.Int).Add(field.Modulus(), big.NewInt(1))
	halfBig.Rsh(halfBig, 1)

	for _, rq := range resolved {
		xInv := rq.XInv
		for j := range p.numRounds() {
			self, sib := rq.Rounds[j].Self, rq.Rounds[j].Sibling
			if levelPair, ok := rq.Aux[j]; ok {
				self, sib = levelPair.Self, levelPair.Sibling
			}
			sum := api.AddExt(self, sib)
			diff := api.SubExt(self, sib)
			diff = api.MulByFpExt(diff, xInv)
			diff = api.MulExt(diff, foldAlphas[j])
			sum = api.AddExt(sum, diff)
			sum = api.MulConstExt(sum, halfBig)

			if j < p.numRounds()-1 {
				api.AssertIsEqualExt(sum, rq.Rounds[j+1].Self)
			} else {
				api.AssertIsEqualExt(sum, rq.Final)
			}
			xInv = api.Mul(xInv, xInv)
		}
		if p.numRounds() > 0 {
			if pair, ok := rq.Aux[p.numRounds()]; ok {
				api.AssertIsEqualExt(pair.Self, pair.Sibling)
			}
		}
	}
}

// =============================================================================
// Query reconstruction
// =============================================================================

// reconstructQueryValueAtGnark mirrors [reconstructQueryValueAt].
func reconstructQueryValueAtGnark(
	api *circuit.KoalaBearAPI,
	bundle sizeBundle,
	entryClaims [][]gnarkQuotientClaim,
	opening GnarkInputQuery,
	inputIndexByBatch []int,
	levelSize int,
	alphaDeep circuit.Ext,
	x circuit.Ext,
	sibling bool,
	running circuit.Ext,
) circuit.Ext {
	value := running
	for i := len(bundle.Entries) - 1; i >= 0; i-- {
		entry := bundle.Entries[i]
		branch := opening[inputIndexByBatch[entry.BatchIdx]]
		pair := pairAtLevelGnark(branch, levelSize)
		row := pair[0]
		if sibling {
			row = pair[1]
		}
		entryValue := rowValueGnark(api, row, entry)
		term := quotientAtValueGnark(api, entryValue, x, entryClaims[i])
		value = api.MulExt(value, alphaDeep)
		value = api.AddExt(value, term)
	}
	return value
}

func rowValueGnark(api *circuit.KoalaBearAPI, row GnarkRowOpening, entry deepEntry) circuit.Ext {
	if entry.IsExt {
		return row.Ext[entry.RowIdx]
	}
	return api.FromBaseExt(row.Base[entry.RowIdx])
}

// quotientAtValueGnark mirrors [quotientAtValue]. A claim point equal to the
// query point makes the division unsatisfiable, matching the native rejection.
func quotientAtValueGnark(api *circuit.KoalaBearAPI, value, x circuit.Ext, claims []gnarkQuotientClaim) circuit.Ext {
	res := api.ZeroExt()
	for _, claim := range claims {
		numerator := api.SubExt(value, claim.Value)
		denominator := api.SubExt(x, claim.Point)
		res = api.AddExt(res, api.DivExt(numerator, denominator))
	}
	return res
}

// bindInputTreeOpeningsGnark mirrors [bindInputTreeOpenings]; it is purely
// structural so every failure is a panic.
func bindInputTreeOpeningsGnark(
	opening GnarkInputQuery, inputIndexByBatch []int,
	levelSize int, order []int, bundle sizeBundle, shapes []Shape,
) {
	for _, batchIdx := range order {
		branchIdx := inputIndexByBatch[batchIdx]
		if branchIdx < 0 || branchIdx >= len(opening) {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: batch %d has no input opening", batchIdx))
		}
		pair := pairAtLevelGnark(opening[branchIdx], levelSize)
		shape := shapes[batchIdx][bundle.SizeLog2]
		for s := range pair {
			if len(pair[s].Base) != shape.BaseWidth || len(pair[s].Ext) != shape.ExtWidth {
				panic(fmt.Sprintf("fri: pcs.VerifyGnark: tree %d row shape mismatch at size %d", branchIdx, levelSize))
			}
		}
	}
}

// pairAtLevelGnark mirrors [InputTreeOpening.pairAtLevel].
func pairAtLevelGnark(branch GnarkInputTreeOpening, levelSize int) GnarkRowPair {
	levelIdx, err := levelIndexGnark(len(branch.Leaves), levelSize)
	if err != nil {
		panic(err)
	}
	pair := branch.Leaves[levelIdx]
	if pair[0].isAbsent() {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: levelSize %d is absent from branch", levelSize))
	}
	return pair
}

// levelIndexGnark mirrors [InputTreeOpening.levelIndex] given the number of
// levels of the branch.
func levelIndexGnark(numLevels, levelSize int) (int, error) {
	if levelSize <= 0 || levelSize&(levelSize-1) != 0 {
		return 0, fmt.Errorf("levelSize must be a positive power of two")
	}
	treeLeaves := 1 << numLevels
	if levelSize > treeLeaves {
		return 0, fmt.Errorf("levelSize %d exceeds branch tree size %d", levelSize, treeLeaves)
	}
	if levelSize == treeLeaves {
		return numLevels - 1, nil
	}
	levelLog := bits.TrailingZeros(uint(levelSize)) - 1
	if levelLog < 0 {
		return 0, fmt.Errorf("levelSize %d has no aux sibling in branch", levelSize)
	}
	return levelLog, nil
}

// =============================================================================
// Merkle authentication
// =============================================================================

// authenticateInputQueryGnark mirrors [authenticateInputQuery].
func authenticateInputQueryGnark(
	api *circuit.KoalaBearAPI, p Params, opening GnarkInputQuery,
	roots []poseidon2.KoalagnarkOctuplet, posBits []frontend.Variable,
) {
	if len(opening) != len(roots) {
		panic(fmt.Sprintf("fri: pcs.VerifyGnark: input query has %d tree openings, want %d", len(opening), len(roots)))
	}
	for i, branch := range opening {
		numLevels := len(branch.Leaves)
		if numLevels == 0 || branch.Leaves[numLevels-1][0].isAbsent() {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: input tree %d: missing bottom level", i))
		}
		if numLevels > int(p.LogCodewordSize) {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: input tree %d: tree deeper than the codeword domain", i))
		}
		if len(branch.Siblings) != numLevels-1 {
			panic(fmt.Sprintf("fri: pcs.VerifyGnark: input tree %d: malformed branch", i))
		}
		// leafIndex = position / (codewordSize / numLeaves): drop the low bits.
		idxBits := posBits[int(p.LogCodewordSize)-numLevels:]
		root := recoverInputRootGnark(api, branch, idxBits)
		assertOctupletEqual(api, root, roots[i])
	}
}

// recoverInputRootGnark mirrors [InputTreeOpening.RecoverRoot]. idxBits are the
// little-endian bits of the leaf index, one per level.
func recoverInputRootGnark(
	api *circuit.KoalaBearAPI, branch GnarkInputTreeOpening, idxBits []frontend.Variable,
) poseidon2.KoalagnarkOctuplet {
	numLevels := len(branch.Leaves)
	bottom := branch.Leaves[numLevels-1]
	ancestor := hashRowOpeningGnark(api, bottom[0])
	sibling := hashRowOpeningGnark(api, bottom[1])
	ancestor = foldOneLevelGnark(api, ancestor, sibling, nil, idxBits[0])

	for i := numLevels - 2; i >= 0; i-- {
		var aux *GnarkRowPair
		if !branch.Leaves[i][0].isAbsent() {
			aux = &branch.Leaves[i]
		}
		ancestor = foldOneLevelGnark(api, ancestor, branch.Siblings[i], aux, idxBits[numLevels-1-i])
	}
	return ancestor
}

// recoverRootGnark mirrors [Branch.RecoverRoot] for aux-free running trees.
func recoverRootGnark(
	api *circuit.KoalaBearAPI, branch GnarkBranch, idxBits []frontend.Variable,
) poseidon2.KoalagnarkOctuplet {
	ancestor := branch.Leaf
	n := len(branch.Siblings)
	for i := n - 1; i >= 0; i-- {
		ancestor = foldOneLevelGnark(api, ancestor, branch.Siblings[i], nil, idxBits[n-1-i])
	}
	return ancestor
}

// foldOneLevelGnark mirrors [foldOneLevel]. isOdd is the current position bit:
// when set, the ancestor is the right child.
func foldOneLevelGnark(
	api *circuit.KoalaBearAPI, ancestor, sibling poseidon2.KoalagnarkOctuplet, aux *GnarkRowPair, isOdd frontend.Variable,
) poseidon2.KoalagnarkOctuplet {
	var left, right poseidon2.KoalagnarkOctuplet
	for i := range left {
		left[i] = api.Select(isOdd, sibling[i], ancestor[i])
		right[i] = api.Select(isOdd, ancestor[i], sibling[i])
	}
	res := poseidon2.KoalagnarkCompress(api, left, right)
	if aux != nil {
		auxDigest := hashAuxPairGnark(api, *aux, isOdd)
		res = poseidon2.KoalagnarkCompress(api, res, auxDigest)
	}
	return res
}

// hashRowOpeningGnark mirrors [hashRowOpening].
func hashRowOpeningGnark(api *circuit.KoalaBearAPI, row GnarkRowOpening) poseidon2.KoalagnarkOctuplet {
	h := poseidon2.NewKoalagnarkMDHasher(api.Frontend())
	absorbLeafHeaderGnark(api, h, len(row.Base), len(row.Ext))
	h.Write(rowOpeningElementsGnark(row)...)
	return h.Sum()
}

// hashAuxPairGnark mirrors [hashAuxPair]: rows are absorbed even-first, so the
// order depends on the (variable) position bit and is resolved element-wise.
func hashAuxPairGnark(
	api *circuit.KoalaBearAPI, pair GnarkRowPair, isOdd frontend.Variable,
) poseidon2.KoalagnarkOctuplet {
	h := poseidon2.NewKoalagnarkMDHasher(api.Frontend())
	absorbLeafHeaderGnark(api, h, len(pair[0].Base), len(pair[0].Ext))
	first := rowOpeningElementsGnark(pair[0])
	second := rowOpeningElementsGnark(pair[1])
	if len(first) != len(second) {
		panic("fri: pcs.VerifyGnark: conjugate rows have different widths")
	}
	seq := make([]circuit.Element, 0, 2*len(first))
	for i := range first {
		seq = append(seq, api.Select(isOdd, second[i], first[i]))
	}
	for i := range second {
		seq = append(seq, api.Select(isOdd, first[i], second[i]))
	}
	h.Write(seq...)
	return h.Sum()
}

// absorbLeafHeaderGnark mirrors [absorbLeafHeader].
func absorbLeafHeaderGnark(api *circuit.KoalaBearAPI, h *poseidon2.KoalagnarkMDHasher, baseWidth, extWidth int) {
	var tag field.Element
	tag.SetUint64(leafDomainTag)
	h.Write(
		api.ConstBig(tag.BigInt(new(big.Int))),
		api.Const(int64(baseWidth)),
		api.Const(int64(extWidth)),
	)
}

// rowOpeningElementsGnark mirrors [writeRowOpeningElements]: base values then
// the six coordinates of every extension value.
func rowOpeningElementsGnark(row GnarkRowOpening) []circuit.Element {
	res := make([]circuit.Element, 0, len(row.Base)+6*len(row.Ext))
	res = append(res, row.Base...)
	for _, e := range row.Ext {
		b0a0, b0a1, b1a0, b1a1, b2a0, b2a1 := e.Coordinates()
		res = append(res, b0a0, b0a1, b1a0, b1a1, b2a0, b2a1)
	}
	return res
}

func assertOctupletEqual(api *circuit.KoalaBearAPI, a, b poseidon2.KoalagnarkOctuplet) {
	for i := range a {
		api.AssertIsEqual(a[i], b[i])
	}
}

// octupletToExtGnark mirrors [octupletToExt]: coordinates 6 and 7 must be zero.
func octupletToExtGnark(api *circuit.KoalaBearAPI, o poseidon2.KoalagnarkOctuplet) circuit.Ext {
	api.AssertIsEqual(o[6], api.Zero())
	api.AssertIsEqual(o[7], api.Zero())
	return circuit.Ext{
		B0: circuit.E2{A0: o[0], A1: o[1]},
		B1: circuit.E2{A0: o[2], A1: o[3]},
		B2: circuit.E2{A0: o[4], A1: o[5]},
	}
}

// =============================================================================
// Domain points
// =============================================================================

// domainPointGnark mirrors [domainPointExt]: the domain point at a
// bit-reversed position, given the position's little-endian bits (at least
// log2(cardinality) of them; extra high bits are ignored, matching the native
// masking through bitReverseExponent). Position bit i is exponent bit
// logSize-1-i, so the point is a product of selected constant powers.
func domainPointGnark(api *circuit.KoalaBearAPI, domain domainLight, posBits []frontend.Variable) circuit.Ext {
	return api.FromBaseExt(domainPointBaseGnark(api, domain.generator, domain.cardinality, posBits))
}

// domainPointInvGnark returns the inverse of [domainPointGnark], built from the
// inverse generator so no in-circuit inversion is needed.
func domainPointInvGnark(api *circuit.KoalaBearAPI, domain domainLight, posBits []frontend.Variable) circuit.Element {
	var genInv field.Element
	genInv.Inverse(&domain.generator)
	return domainPointBaseGnark(api, genInv, domain.cardinality, posBits)
}

func domainPointBaseGnark(
	api *circuit.KoalaBearAPI, generator field.Element, cardinality uint64, posBits []frontend.Variable,
) circuit.Element {
	logSize := bits.TrailingZeros64(cardinality)
	if len(posBits) < logSize {
		panic(fmt.Sprintf("fri: domainPointGnark: %d position bits for a domain of log-size %d", len(posBits), logSize))
	}
	one := api.One()
	acc := one
	pow := generator // generator^(2^e), for e = 0, 1, ...
	for e := 0; e < logSize; e++ {
		// exponent bit e is position bit logSize-1-e
		bit := posBits[logSize-1-e]
		factor := api.Select(bit, api.ConstBig(pow.BigInt(new(big.Int))), one)
		acc = api.Mul(acc, factor)
		pow.Square(&pow)
	}
	return acc
}
