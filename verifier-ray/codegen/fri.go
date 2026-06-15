package codegen

import "github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"

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
