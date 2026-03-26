package distributed

import (
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/dlclark/regexp2"
)

// ModuleDiscoveryAdvice is an advice provided by the user and allows having
// fine-grained control over the assignment of columns to modules and their
// sizes.
type ModuleDiscoveryAdvice struct {
	Column    ifaces.Column
	Regexp    string
	ModuleRef string
	Cluster   ModuleName
	BaseSize  int
	rgxp      *regexp2.Regexp `serde:"omit"`
}

// SameSizeAdvice returns an advice from a column where the base-size equals
// the one of the provided column.
func SameSizeAdvice(cls ModuleName, column ifaces.Column) *ModuleDiscoveryAdvice {
	return &ModuleDiscoveryAdvice{Column: column, Cluster: cls, BaseSize: column.Size()}
}

// DoesMatch returns true if the present advices matches the provided column.
func (ad *ModuleDiscoveryAdvice) DoesMatch(column ifaces.Column) bool {

	ad.assertWellFormed()

	if ad.Column != nil {
		// We should not have two columns with the same column ID. So that's a
		// valid and less-edge-case-ish way to compare two columns.
		return ad.Column.GetColID() == column.GetColID()
	}

	if len(ad.ModuleRef) > 0 {
		modRef, ok := pragmas.TryGetModuleRef(column)
		if !ok {
			return false
		}
		return modRef == ad.ModuleRef
	}

	// Assertedly, the regexp is provided and we already checked that in
	// [assertWellFormed].
	if ad.rgxp == nil {
		rgxp, err := regexp2.Compile(ad.Regexp, regexp2.Singleline)
		if err != nil {
			utils.Panic("failed to compile regexp: %++v", err)
		}
		ad.rgxp = rgxp
	}

	res, err := ad.rgxp.MatchString(string(column.GetColID()))
	if err != nil {
		utils.Panic("failed to match regexp: %s", err.Error())
	}

	return res
}

// AreConflicting returns true if the two advices conflict. Namely, if their
// BaseSize or Cluster differ.
func (ad ModuleDiscoveryAdvice) AreConflicting(other *ModuleDiscoveryAdvice) bool {
	return ad.BaseSize != other.BaseSize || ad.Cluster != other.Cluster
}

// assertWellFormeded sanity-checks if the advice is well-formed.
func (ad ModuleDiscoveryAdvice) assertWellFormed() error {

	if ad.Column == nil && len(ad.Regexp) == 0 && len(ad.ModuleRef) == 0 {
		utils.Panic("advice does not specify a column or a regexp: %++v", ad)
	}

	if !utils.IsPowerOfTwo(ad.BaseSize) {
		utils.Panic("advice does not specify a base size that is a power of two, %++v", ad)
	}

	if len(ad.Cluster) == 0 {
		utils.Panic("advice does not specify a cluster: %++v", ad)
	}

	return nil
}
