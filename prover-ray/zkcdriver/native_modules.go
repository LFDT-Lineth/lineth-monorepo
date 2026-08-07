package zkcdriver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/zkc/pkg/schema"
	"github.com/LFDT-Lineth/zkc/pkg/schema/register"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
)

const (
	nativeMulmod = "mulmod_"
	expmodHint   = "expmod_unconstrained_"

	nativeMulmodLeft      = "lhsL'"
	nativeMulmodRight     = "rhsL'"
	nativeMulmodModulus   = "modL'"
	nativeMulmodRemainder = "remainder'"
	nativeMulmodQuotient  = "quotient'"

	expmodBase = "base'"
	expmodExp  = "exp'"
	expmodMod  = "mod'"
	expmodRes  = "res'"
)

func (s *schemaScanner) defineNativeModule(mod schema.Module[koalabear.Element]) error {
	modName := mod.Name().String()
	if after, found := strings.CutPrefix(modName, nativeMulmod); found {
		if err := s.defineNativeMulmod(mod, after); err != nil {
			return fmt.Errorf("native mulmod %s: %w", modName, err)
		}
		return nil
	}
	if strings.HasPrefix(modName, expmodHint) {
		// expmod_unconstrained_* is unconstrained. Though, we still need to register the columns so that lookups would work.
		s.defineNativeExpUnconstrained(mod)
		return nil
	}
	return fmt.Errorf("unknown native module %s", modName)
}

// defineNativeExpUnconstrained registers a wiop column for known register of a
// native module without adding any constraint, leaving the module genuinely
// unconstrained while still giving callers a valid column to reference.
func (s *schemaScanner) defineNativeExpUnconstrained(mod schema.Module[koalabear.Element]) {
	modName := mod.Name().String()

	moduleWIOP := s.Sys.NewDynamicModule(
		s.Sys.Context.Childf("module-native-%s", modName),
		wiop.PaddingDirectionRight,
	)

	for _, reg := range mod.Registers() {
		matched := false
		for _, prefix := range []string{
			expmodBase, expmodExp, expmodMod, expmodRes,
		} {
			if _, found := strings.CutPrefix(reg.Name(), prefix); found {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		colQualifiedName := qualifiedCorsetName(modName, reg.Name())
		col := moduleWIOP.NewColumn(
			s.Sys.Context.Childf("col-native-%s", colQualifiedName),
			wiop.VisibilityOracle,
			s.Sys.Rounds[0],
		)
		s.ColumnIDs[colQualifiedName] = col.Context.ID
	}
}

func (s *schemaScanner) defineNativeMulmod(mod schema.Module[koalabear.Element], after string) error {
	nbBits, err := strconv.Atoi(after)
	if err != nil {
		return fmt.Errorf("invalid number of bits: %w", err)
	}
	if nbBits <= 0 {
		return fmt.Errorf("invalid number of bits %d: must be positive", nbBits)
	}

	modName := mod.Name().String()

	// nbBitsPerLimb is the bitwidth of one limb register, as declared by the
	// schema. All limb registers of a mulmod module share the same width.
	nbBitsPerLimb := 0
	for _, reg := range mod.Registers() {
		matched := false
		for _, prefix := range []string{
			nativeMulmodLeft, nativeMulmodRight, nativeMulmodModulus,
			nativeMulmodRemainder, nativeMulmodQuotient,
		} {
			if _, found := strings.CutPrefix(reg.Name(), prefix); found {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		w := int(reg.Width())
		if nbBitsPerLimb == 0 {
			nbBitsPerLimb = w
			continue
		}
		if w != nbBitsPerLimb {
			return fmt.Errorf("inconsistent limb width: register %q has width %d, expected %d", reg.Name(), w, nbBitsPerLimb)
		}
	}
	if nbBitsPerLimb == 0 {
		return fmt.Errorf("could not determine limb width: no limb register found in module %s", modName)
	}
	// Compute the number of limbs required to represent the given number of bits
	nbLimbs := (nbBits + nbBitsPerLimb - 1) / nbBitsPerLimb

	moduleWIOP := s.Sys.NewDynamicModule(
		s.Sys.Context.Childf("module-native-%s", modName),
		wiop.PaddingDirectionRight,
	)

	// look up the limbs from the traces
	leftLimbs := make([]*wiop.Column, nbLimbs)
	rightLimbs := make([]*wiop.Column, nbLimbs)
	modulusLimbs := make([]*wiop.Column, nbLimbs)
	resultLimbs := make([]*wiop.Column, nbLimbs)
	quotientLimbs := make([]*wiop.Column, nbLimbs)

	assignLimb := func(limbs []*wiop.Column, prefix string, reg register.Register) error {
		idxS, found := strings.CutPrefix(reg.Name(), prefix)
		if !found {
			// not something we care about, ignore
			return nil
		}
		idx, err := strconv.Atoi(idxS)
		if err != nil {
			return fmt.Errorf("invalid limb index %q: %w", idxS, err)
		}
		if idx < 0 || idx >= nbLimbs {
			return fmt.Errorf("limb index %d out of range [0, %d)", idx, nbLimbs)
		}
		colQualifiedName := qualifiedCorsetName(modName, reg.Name())
		col := moduleWIOP.NewColumn(
			s.Sys.Context.Childf("col-native-%s", colQualifiedName),
			wiop.VisibilityOracle,
			s.Sys.Rounds[0],
		)
		limbs[idx] = col
		s.ColumnIDs[colQualifiedName] = col.Context.ID
		return nil
	}

	regs := mod.Registers()
	for i := range regs {
		for _, j := range []struct {
			limbs  []*wiop.Column
			prefix string
		}{
			{leftLimbs, nativeMulmodLeft},
			{rightLimbs, nativeMulmodRight},
			{modulusLimbs, nativeMulmodModulus},
			{resultLimbs, nativeMulmodRemainder},
			{quotientLimbs, nativeMulmodQuotient},
		} {
			if err := assignLimb(j.limbs, j.prefix, regs[i]); err != nil {
				return fmt.Errorf("assign limb: %w", err)
			}
		}
	}
	// check that all limbs have been assigned
	for _, j := range []struct {
		limbs  []*wiop.Column
		prefix string
	}{
		{leftLimbs, nativeMulmodLeft},
		{rightLimbs, nativeMulmodRight},
		{modulusLimbs, nativeMulmodModulus},
		{resultLimbs, nativeMulmodRemainder},
		{quotientLimbs, nativeMulmodQuotient},
	} {
		for i := range j.limbs {
			if j.limbs[i] == nil {
				return fmt.Errorf("missing limb %d for prefix %q", i, j.prefix)
			}
		}
	}

	moduleWIOP.NewNonNative(
		s.Sys.Context.Childf("nonnative-%s", modName),
		nbBitsPerLimb,
		leftLimbs, rightLimbs, modulusLimbs, resultLimbs, quotientLimbs,
	)

	return nil
}
