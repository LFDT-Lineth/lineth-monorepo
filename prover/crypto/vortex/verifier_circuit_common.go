package vortex

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/maths/polynomials"
)

type GnarkProof struct {
	Columns           [][][]koalagnark.Element
	LinearCombination []koalagnark.Ext
}

type GnarkVerifierInput struct {

	// alpha random coin used for the linear combination
	Alpha koalagnark.Ext

	// X is the univariate evaluation point
	X koalagnark.Ext

	// Ys are the alleged evaluation at point X
	Ys [][]koalagnark.Ext

	// EntryList is the random coin representing the columns to open.
	EntryList []frontend.Variable
}

func GnarkVerify(api frontend.API, fs fiatshamir.GnarkFS, params Params, proof GnarkProof, vi GnarkVerifierInput) error {

	err := GnarkCheckLinComb(api, proof.LinearCombination, vi.EntryList, vi.Alpha, proof.Columns)
	if err != nil {
		return err
	}

	// Batch the two Lagrange evaluations (GnarkCheckStatement and GnarkCheckIsCodeWord)
	// since they both evaluate the same polynomial on the same domain
	err = GnarkCheckStatementAndCodeWord(api, fs, params, proof.LinearCombination, vi.Ys, vi.X, vi.Alpha)
	return err
}

// GnarkCheckStatementAndCodeWord combines GnarkCheckStatement and GnarkCheckIsCodeWord
// to share the Lagrange basis computation, saving approximately n multiplications.
func GnarkCheckStatementAndCodeWord(
	api frontend.API,
	fs fiatshamir.GnarkFS,
	params Params,
	linComb []koalagnark.Ext,
	ys [][]koalagnark.Ext,
	x, alpha koalagnark.Ext) error {

	koalaAPI := koalagnark.NewAPI(api)

	// === Part 1: Prepare for codeword check (compute FFT inverse via hint) ===
	fftinv := fftInvHint(koalaAPI.Type())
	sizeFextUnpacked := len(linComb) * 4
	inputs := make([]koalagnark.Element, sizeFextUnpacked)
	for i := 0; i < len(linComb); i++ {
		inputs[4*i] = linComb[i].B0.A0
		inputs[4*i+1] = linComb[i].B0.A1
		inputs[4*i+2] = linComb[i].B1.A0
		inputs[4*i+3] = linComb[i].B1.A1
	}
	resLen := 4 * params.RsParams.NbColumns()
	_res, err := koalaAPI.NewHint(fftinv, resLen, inputs...)
	if err != nil {
		return err
	}

	res := make([]koalagnark.Ext, resLen/4)
	for i := range res {
		res[i].B0.A0 = _res[4*i]
		res[i].B0.A1 = _res[4*i+1]
		res[i].B1.A0 = _res[4*i+2]
		res[i].B1.A1 = _res[4*i+3]
	}
	fs.UpdateExt(res...)
	challenge := fs.RandomFieldExt()

	// === Part 2: Codeword check (Schwartz-Zippel) ===
	// Evaluate linComb at alpha (for codeword check).
	// Alpha is used as the evaluation point for the fft AND for the folding.
	evalLag := polynomials.GnarkEvaluateLagrangeExt(
		api,
		linComb,
		challenge,
		params.RsParams.Domains[1].Generator,
		params.RsParams.Domains[1].Cardinality)

	evalCan := polynomials.GnarkEvalCanonicalExt(api, res, challenge)
	koalaAPI.AssertIsEqualExt(evalLag, evalCan)

	// === Part 3: Assert last entries are zeroes (RS codeword property) ===

	// this is already implied by the Schwartz-Zippel above as the FFT Inverse hint outputs
	// a polynomial of degree < nbColumns, so we skip this step

	// === Part 4: Statement check ===
	alphaY := polynomials.GnarkEvalCanonicalExt(api, res, x)

	var yjoined []koalagnark.Ext
	for i := 0; i < len(ys); i++ {
		yjoined = append(yjoined, ys[i]...)
	}
	alphaYPrime := polynomials.GnarkEvalCanonicalExt(api, yjoined, alpha)
	koalaAPI.AssertIsEqualExt(alphaY, alphaYPrime)

	return nil
}

func GnarkCheckLinComb(
	api frontend.API, linComb []koalagnark.Ext,
	entryList []frontend.Variable, alpha koalagnark.Ext,
	columns [][][]koalagnark.Element) error {

	koalaAPI := koalagnark.NewAPI(api)

	numCommitments := len(columns)

	// Prepare all polynomials for batch evaluation at alpha ===
	// Collect all fullCol polynomials first
	fullCols := make([][]koalagnark.Element, len(entryList))

	for j := range entryList {

		// Will carry the concatenation of the columns for the same entry j
		fullCol := []koalagnark.Element{}

		for i := range numCommitments {
			// Entries of the selected columns #j contained in the commitment #i.
			fullCol = append(fullCol, columns[i][j]...)
		}

		fullCols[j] = fullCol
	}
	ys := polynomials.GnarkEvalCanonicalBatch(api, fullCols, alpha)
	table1 := logderivlookup.New(api)
	table2 := logderivlookup.New(api)
	table3 := logderivlookup.New(api)
	table4 := logderivlookup.New(api)

	for k := range linComb {
		table1.Insert(linComb[k].B0.A0.Native())
		table2.Insert(linComb[k].B0.A1.Native())
		table3.Insert(linComb[k].B1.A0.Native())
		table4.Insert(linComb[k].B1.A1.Native())
	}

	v1 := table1.Lookup(entryList...)
	v2 := table2.Lookup(entryList...)
	v3 := table3.Lookup(entryList...)
	v4 := table4.Lookup(entryList...)

	// Construct the lookup results
	lookedUpValues := make([]koalagnark.Ext, len(entryList))
	for j := range entryList {
		lookedUpValues[j].B0.A0 = koalagnark.WrapFrontendVariable(v1[j])
		lookedUpValues[j].B0.A1 = koalagnark.WrapFrontendVariable(v2[j])
		lookedUpValues[j].B1.A0 = koalagnark.WrapFrontendVariable(v3[j])
		lookedUpValues[j].B1.A1 = koalagnark.WrapFrontendVariable(v4[j])
	}

	// Compare with ys
	for j := range entryList {
		koalaAPI.AssertIsEqualExt(ys[j], lookedUpValues[j])
	}

	return nil
}
