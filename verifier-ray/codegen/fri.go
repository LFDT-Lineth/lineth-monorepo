package codegen

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Slot identifies a polynomial's position within the commitment scheme:
// which tree it is in, which index within that tree's polynomial list, and
// which rail (base or extension field).
type Slot struct {
	TreeIdx int
	PolyIdx int
	Rail    field.Kind
}

// Layout holds the program-determined tree layout, validated and ready for Zig
// rendering.
type Layout struct {
	NumTrees      int             // total number of commitment trees in the proof layout
	SetupBegin    int             // first setup commitment tree index
	SetupEnd      int             // exclusive end of setup commitment tree indices
	TraceBegin    []int           // per trace round, first trace commitment tree index
	TraceEnd      []int           // per trace round, exclusive end of trace commitment tree indices
	AirBegin      int             // first AIR quotient commitment tree index
	AirEnd        int             // exclusive end of AIR quotient commitment tree indices
	TreeSizes     []int           // tree_sizes[tree_idx] is the committed domain size
	ColSlots      map[string]Slot // column name to proof slot
	AirChunkSlots map[string]Slot // AIR chunk name to proof slot
}

// BuildLayout validates cfg and returns the Layout. It checks structural
// invariants: slice lengths must match NumTrees, and every slot's TreeIdx must
// be in [0, NumTrees).
func BuildLayout(cfg Layout) (Layout, error) {
	if len(cfg.TreeSizes) != cfg.NumTrees {
		return Layout{}, fmt.Errorf("fri codegen: TreeSizes length %d != NumTrees %d",
			len(cfg.TreeSizes), cfg.NumTrees)
	}
	if len(cfg.TraceBegin) != len(cfg.TraceEnd) {
		return Layout{}, fmt.Errorf("fri codegen: TraceBegin length %d != TraceEnd length %d",
			len(cfg.TraceBegin), len(cfg.TraceEnd))
	}
	for name, slot := range cfg.ColSlots {
		if slot.TreeIdx >= cfg.NumTrees {
			return Layout{}, fmt.Errorf("fri codegen: column %q slot tree_idx %d >= num_trees %d",
				name, slot.TreeIdx, cfg.NumTrees)
		}
	}
	for name, slot := range cfg.AirChunkSlots {
		if slot.TreeIdx >= cfg.NumTrees {
			return Layout{}, fmt.Errorf("fri codegen: air chunk %q slot tree_idx %d >= num_trees %d",
				name, slot.TreeIdx, cfg.NumTrees)
		}
	}
	return Layout{
		NumTrees:      cfg.NumTrees,
		SetupBegin:    cfg.SetupBegin,
		SetupEnd:      cfg.SetupEnd,
		TraceBegin:    append([]int(nil), cfg.TraceBegin...),
		TraceEnd:      append([]int(nil), cfg.TraceEnd...),
		AirBegin:      cfg.AirBegin,
		AirEnd:        cfg.AirEnd,
		TreeSizes:     append([]int(nil), cfg.TreeSizes...),
		ColSlots:      cfg.ColSlots,
		AirChunkSlots: cfg.AirChunkSlots,
	}, nil
}

// ColRef identifies a column by its prover-ray source name and its protocol
// key (the string used in ValuesAtZeta).
type ColRef struct {
	Name string
	Key  string
}

// DQLevel holds the DEEP-quotient structure for one domain size. Evaluation
// points are encoded as shift exponents rather than field elements: the actual
// evaluation point for shift k is ω_N^k · ζ, where ζ is the out-of-domain
// challenge derived at transcript-replay time. This keeps DQLevel fully
// program-determined.
type DQLevel struct {
	Size      int        // polynomial domain size for this level (must be a positive power of two)
	Shifts    []int      // Shifts[i]: exponent k such that eval point = ω_N^k · ζ
	ColGroups [][]ColRef // ColGroups[i]: columns evaluated at the point with Shifts[i]
	AirChunks []string   // AIR chunk names at this domain size
}

// DQLayout holds the DEEP-quotient structure for all distinct domain sizes.
type DQLayout struct {
	Levels []DQLevel
}

// BuildDQLayout validates levels and returns the DQLayout. It checks that
// every level's Size is a positive power of two, and that Shifts and ColGroups
// have the same length for each level.
func BuildDQLayout(levels []DQLevel) (DQLayout, error) {
	for i, lv := range levels {
		if lv.Size <= 0 || lv.Size&(lv.Size-1) != 0 {
			return DQLayout{}, fmt.Errorf("fri codegen: level %d size %d is not a positive power of two", i, lv.Size)
		}
		if len(lv.Shifts) != len(lv.ColGroups) {
			return DQLayout{}, fmt.Errorf("fri codegen: level %d: Shifts length %d != ColGroups length %d",
				i, len(lv.Shifts), len(lv.ColGroups))
		}
	}
	return DQLayout{Levels: append([]DQLevel(nil), levels...)}, nil
}
