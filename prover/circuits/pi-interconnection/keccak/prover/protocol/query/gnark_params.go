package query

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
)

// A gnark circuit version of the LocalOpeningResult
type GnarkLocalOpeningParams struct {
	Y frontend.Variable
}

func (p LocalOpeningParams) GnarkAssign() GnarkLocalOpeningParams {
	return GnarkLocalOpeningParams{Y: p.Y}
}

// A gnark circuit version of LogDerivSumParams
type GnarkLogDerivSumParams struct {
	Sum frontend.Variable
}

// A gnark circuit version of GrandProductParams
type GnarkGrandProductParams struct {
	Prod frontend.Variable
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
	FinalResult frontend.Variable
	// Parts are the parameters of the Horner parts
	Parts []HornerParamsPartGnark
}

func (p LogDerivSumParams) GnarkAssign() GnarkLogDerivSumParams {
	return GnarkLogDerivSumParams{Sum: p.Sum}
}

// A gnark circuit version of InnerProductParams
type GnarkInnerProductParams struct {
	Ys []frontend.Variable
}

func (p InnerProduct) GnarkAllocate() GnarkInnerProductParams {
	return GnarkInnerProductParams{Ys: make([]frontend.Variable, len(p.Bs))}
}

func (p InnerProductParams) GnarkAssign() GnarkInnerProductParams {
	return GnarkInnerProductParams{Ys: vector.IntoGnarkAssignment(p.Ys)}
}

// A gnark circuit version of univariate eval params
type GnarkUnivariateEvalParams struct {
	X  frontend.Variable
	Ys []frontend.Variable
}

func (p UnivariateEval) GnarkAllocate() GnarkUnivariateEvalParams {
	// no need to preallocate the x because its size is already known
	return GnarkUnivariateEvalParams{Ys: make([]frontend.Variable, len(p.Pols))}
}

// Returns a gnark assignment for the present parameters
func (p UnivariateEvalParams) GnarkAssign() GnarkUnivariateEvalParams {
	return GnarkUnivariateEvalParams{Ys: vector.IntoGnarkAssignment(p.Ys), X: p.X}
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
			N0: part.N0,
			N1: part.N1,
		}
	}

	return GnarkHornerParams{
		FinalResult: p.FinalResult,
		Parts:       parts,
	}
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkInnerProductParams) UpdateFS(fs *fiatshamir.GnarkFiatShamir) {
	fs.Update(p.Ys...)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkLocalOpeningParams) UpdateFS(fs *fiatshamir.GnarkFiatShamir) {
	fs.Update(p.Y)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkLogDerivSumParams) UpdateFS(fs *fiatshamir.GnarkFiatShamir) {
	fs.Update(p.Sum)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkGrandProductParams) UpdateFS(fs *fiatshamir.GnarkFiatShamir) {
	fs.Update(p.Prod)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkUnivariateEvalParams) UpdateFS(fs *fiatshamir.GnarkFiatShamir) {
	fs.Update(p.Ys...)
}

// Update the fiat-shamir state with the the present parameters
func (p GnarkHornerParams) UpdateFS(fs *fiatshamir.GnarkFiatShamir) {
	fs.Update(p.FinalResult)

	for _, part := range p.Parts {
		fs.Update(part.N0, part.N1)
	}
}
