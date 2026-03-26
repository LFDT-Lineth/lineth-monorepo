package bls

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

const (
	moduleName = "blsdata"
)

type Group int

const (
	G1 Group = iota
	G2
)

type membership int

const (
	CURVE membership = iota
	GROUP
)

const ROUND_NR = 0

func (g Group) String() string {
	switch g {
	case G1:
		return "G1"
	case G2:
		return "G2"
	default:
		panic("unknown group")
	}
}

func (g Group) StringCurve() string {
	switch g {
	case G1:
		return "C1"
	case G2:
		return "C2"
	default:
		panic("unknown group")
	}
}

func (g Group) StringMap() string {
	switch g {
	case G1:
		return "FP"
	case G2:
		return "FP2"
	default:
		panic("unknown group")
	}
}

func createColFn(comp *wizard.CompiledIOP, rootName string, size int) func(name string) ifaces.Column {
	return func(name string) ifaces.Column {
		return comp.InsertCommit(ROUND_NR, ifaces.ColIDf("%s_%s", rootName, name), size, true)
	}
}
