package query

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// A gnark circuit version of the LocalOpeningResult
type GnarkLocalOpeningParams struct {
	BaseY  koalagnark.Element
	ExtY   koalagnark.Ext
	IsBase bool
}

func (p LocalOpeningParams) GnarkAssign() GnarkLocalOpeningParams {

	exty := koalagnark.NewExt(p.ExtY)
	return GnarkLocalOpeningParams{
		BaseY:  koalagnark.NewElementFromKoala(p.BaseY),
		ExtY:   exty,
		IsBase: p.IsBase,
	}
}

// A gnark circuit version of LogDerivSumParams
type GnarkLogDerivSumParams struct {
	Sum koalagnark.Ext
}

// A gnark circuit version of GrandProductParams
type GnarkGrandProductParams struct {
	Prod koalagnark.Ext
}

// HornerParamsPartGnark is a [HornerParamsPart] in a gnark circuit.
type HornerParamsPartGnark struct {
	// N0 is an initial offset of the Horner query
	N0 frontend.Variable
	// N1 is the second offset of the Horner query
	N1 frontend.Variable
}

// GnarkHornerParams represents the parameters of the Horner evaluation query
// in a gnark circuit.
type GnarkHornerParams struct {
	// Final result is the result of summing the Horner parts for every
	// queries.
	FinalResult koalagnark.Ext
	// Parts are the parameters of the Horner parts
	Parts []HornerParamsPartGnark
}

func (p LogDerivSumParams) GnarkAssign() GnarkLogDerivSumParams {
	// return GnarkLogDerivSumParams{Sum: p.Sum}
	tmp := p.Sum.GetExt()
	return GnarkLogDerivSumParams{Sum: koalagnark.NewExt(tmp)} // TODO @thomas fixme (ext vs base)
}

// A gnark circuit version of InnerProductParams
type GnarkInnerProductParams struct {
	Ys []koalagnark.Ext
}

func (p InnerProduct) GnarkAllocate() GnarkInnerProductParams {
	return GnarkInnerProductParams{Ys: make([]koalagnark.Ext, len(p.Bs))}
}

func (p InnerProductParams) GnarkAssign() GnarkInnerProductParams {
	return GnarkInnerProductParams{Ys: vectorext.IntoGnarkAssignment(p.Ys)}
}

// A gnark circuit version of univariate eval params
type GnarkUnivariateEvalParams struct {
	ExtX  koalagnark.Ext
	ExtYs []koalagnark.Ext
}

func (p UnivariateEval) GnarkAllocate() GnarkUnivariateEvalParams {
	// no need to preallocate the x because its size is already known
	return GnarkUnivariateEvalParams{
		ExtYs: make([]koalagnark.Ext, len(p.Pols)),
	}
}

// Returns a gnark assignment for the present parameters
func (p UnivariateEvalParams) GnarkAssign() GnarkUnivariateEvalParams {
	return GnarkUnivariateEvalParams{
		ExtYs: vectorext.IntoGnarkAssignment(p.ExtYs),
		ExtX:  koalagnark.NewExt(p.ExtX),
	}

}

// GnarkAllocate allocates a [GnarkHornerParams] with the right dimensions
func (p HornerParams) GnarkAllocate() GnarkHornerParams {
	return GnarkHornerParams{
		Parts: make([]HornerParamsPartGnark, len(p.Parts)),
	}
}

// GnarkAssign returns a gnark assignment for the present parameters.
func (p HornerParams) GnarkAssign() GnarkHornerParams {

	parts := make([]HornerParamsPartGnark, len(p.Parts))
	for i, part := range p.Parts {
		parts[i] = HornerParamsPartGnark{
			N0: koalagnark.NewElement(part.N0),
			N1: koalagnark.NewElement(part.N1),
		}
	}

	return GnarkHornerParams{
		FinalResult: koalagnark.NewExt(p.FinalResult),
		Parts:       parts,
	}
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkInnerProductParams) UpdateFS(fs fiatshamir.GnarkFS) {
	fs.UpdateExt(p.Ys...)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkLocalOpeningParams) UpdateFS(fs fiatshamir.GnarkFS) {
	if p.IsBase {
		fs.Update(p.BaseY)
	} else {
		fs.UpdateExt(p.ExtY)
	}
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkLogDerivSumParams) UpdateFS(fs fiatshamir.GnarkFS) {
	fs.UpdateExt(p.Sum)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkGrandProductParams) UpdateFS(fs fiatshamir.GnarkFS) {
	fs.UpdateExt(p.Prod)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkUnivariateEvalParams) UpdateFS(fs fiatshamir.GnarkFS) {

	fs.UpdateExt(p.ExtYs...)
}

// Update the fiat-shamir state with the present parameters
func (p GnarkHornerParams) UpdateFS(fs fiatshamir.GnarkFS) {
	fs.UpdateExt(p.FinalResult)

	for _, part := range p.Parts {
		fs.Update(koalagnark.WrapFrontendVariable(part.N0), koalagnark.WrapFrontendVariable(part.N1))
	}
}
