package fft

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Generators of the successive subgroup or roots of unity
var generators []field.Element = initGenerators()

// Computes all the generators of the imbricated subgroups of roots of unity
func initGenerators() []field.Element {

	maxOrder := int(field.RootOfUnityOrder)
	generators := make([]field.Element, maxOrder+1)
	generators[maxOrder] = field.RootOfUnity

	for i := maxOrder - 1; i >= 0; i-- {
		generators[i].Square(&generators[i+1])

		// Sanity-check, we should not have a "one" for unless i == 0
		if i > 0 && generators[i].IsOne() {
			utils.Panic("root_of_unity %v ^ (2 ^ %v) == 1", field.RootOfUnity, maxOrder)
		}
	}

	if !generators[0].IsOne() {
		utils.Panic("root_of_unity %v ^ (2 ^ %v) != 1", field.RootOfUnity, field.RootOfUnityOrder)
	}

	return generators
}

/*
Returns a generator for a domain of application with the requested size.
Omega is a root of unity which generates the domain of evaluation of the
constraint.
*/
func GetOmega(domainSize int) field.Element {

	/*
		We enforce that the passed domainSize
		is a power of two.
	*/
	if !utils.IsPowerOfTwo(domainSize) {
		utils.Panic("Currently, we only support domain sizes that are powers of two, got %v", domainSize)
	}

	/*
		Sanity-check : the domainSize should not excess the
		maximal domain size
	*/
	if domainSize > (1 << field.RootOfUnityOrder) {
		utils.Panic("Required a domain of size %v but the max is %v \n", domainSize, 1<<field.RootOfUnityOrder)
	}

	order := utils.Log2Ceil(domainSize)
	return generators[order]
}
