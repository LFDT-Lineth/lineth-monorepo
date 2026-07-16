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

	nativeMulmodLeft     = "lhsL'"
	nativeMulmodRight    = "rhsL'"
	nativeMulmodModulus  = "modL'"
	nativeMulmodRemainer = "remainder'"
	nativeMulmodQuotient = "quotient'"
)

func (s *schemaScanner) defineNativeModule(mod schema.Module[koalabear.Element]) error {
	modName := mod.Name().String()
	if after, found := strings.CutPrefix(modName, nativeMulmod); found {
		if err := s.defineNativeMulmod(mod, after); err != nil {
			return fmt.Errorf("native mulmod %s: %w", modName, err)
		}
		return nil
	}
	return fmt.Errorf("unknown native module %s", modName)
}

func (s *schemaScanner) defineNativeMulmod(mod schema.Module[koalabear.Element], after string) error {
	nbBits, err := strconv.Atoi(after)
	if err != nil {
		return fmt.Errorf("invalid number of bits: %w", err)
	}

	modName := mod.Name().String()

	// nbBitsPerLimb is the bitwidth of one limb register, as declared by the
	// schema. All limb registers of a mulmod module share the same width.
	nbBitsPerLimb := 0
	for _, reg := range mod.Registers() {
		for _, prefix := range []string{
			nativeMulmodLeft, nativeMulmodRight, nativeMulmodModulus,
			nativeMulmodRemainer, nativeMulmodQuotient,
		} {
			if _, found := strings.CutPrefix(reg.Name(), prefix); found {
				nbBitsPerLimb = int(reg.Width())
				break
			}
		}
		if nbBitsPerLimb != 0 {
			break
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
			{resultLimbs, nativeMulmodRemainer},
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
		{resultLimbs, nativeMulmodRemainer},
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
