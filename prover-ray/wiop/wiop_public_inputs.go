package wiop

import (
	"fmt"
	"strconv"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// PublicInput carries the values of the system's registered public inputs as a
// flat, ordered slice of field elements. Position i holds the value of
// sys.PublicInputs[i], i.e. the i-th cell registered via
// [System.RegisterPublicInputs]. The semantic role of each position is recovered
// from the cell's tag; see [PublicInputTag].
type PublicInput []field.Gen

// PublicInputTagKey is the reserved [Annotations] key under which
// [System.RegisterPublicInputs] stores a cell's [PublicInputTag]. It is owned by
// the public-input machinery: no other code may write it, and a cell already
// carrying it is rejected at registration.
const PublicInputTagKey = "wiop.publicinput.tag"

// publicInputTagSeparator joins a [PublicInputTag] to the numeric suffix
// [System.RegisterPublicInputs] optionally appends to it.
const publicInputTagSeparator = "_"

// PublicInputTag names the semantic role a public-input cell plays, so that a
// position in the flat [PublicInput] vector can be interpreted by callers
// (inter-shard consistency, decoding a value back into a host-side struct,
// error messages) instead of being an anonymous field element.
//
// A tag identifies exactly one public input: [System.RegisterPublicInputs]
// rejects a tag that is already in use. A role spanning several cells (a 32-byte
// hash, say) is therefore registered one cell at a time under a numeric suffix,
// which yields the distinct tags Role_0, Role_1, … — see
// [System.RegisterPublicInputs] and [System.LookupPublicInputByTag].
//
// The set of tags is deliberately open: wiop does not enumerate them, so a new
// role needs no change here. Declare tags as constants next to the code that
// registers them — see the messagebus compiler for an example. Because a
// suffixed tag is just its base tag concatenated with "_" and the index, a
// literal tag of the form Role_0 collides with Role suffixed by 0; pick base
// tags that do not already end in an underscore-separated integer.
//
// A tag is compile-time metadata and is NOT part of the transcript: the verifier
// checks a public input positionally (and through whatever constraint binds the
// cell to its source, e.g. the local constraint that [ColumnPosition.Open]
// registers), never against its tag. Two shards that are joined by position must
// therefore be checked to agree on their tag sequence; a tag is not an
// authenticated claim unless it is separately folded into the verification key.
type PublicInputTag string

// RegisterPublicInputs promotes an already-declared cell to a public input,
// tagging it with tag. It appends the cell to [System.PublicInputs]; the order in
// which cells are registered defines the index-to-cell mapping of the flat
// [PublicInput] vector returned by [System.Prove] and consumed by
// [System.Verify].
//
// If numericSuffix is given, its first element is appended to tag as "_<n>" and
// that suffixed tag is what the cell is tagged and deduplicated under. Any
// further elements are ignored. This is how a role wider than one field element
// is registered — call once per cell with a distinct suffix, yielding Role_0,
// Role_1, … — since tags must be unique and one call registers exactly one cell.
//
// Registration writes the tag into the cell's [Annotations] under
// [PublicInputTagKey]; the cell is otherwise unmodified. Tagging is mandatory
// precisely so that a public input is never anonymous — hence tag must be
// non-empty, and a cell that already carries [PublicInputTagKey] is rejected
// rather than silently re-tagged.
//
// The canonical way to expose a column position is col.At(pos).Open(ctx)
// followed by RegisterPublicInputs on the resulting cell.
//
// Panics if tag is empty, if the resulting tag is already used by another public
// input, or if cell is nil, does not belong to sys, already carries a
// [PublicInputTagKey] annotation, or is already registered.
func (sys *System) RegisterPublicInputs(tag PublicInputTag, cell *Cell, numericSuffix ...int) {
	if tag == "" {
		panic("wiop: RegisterPublicInputs: empty tag")
	}

	if cell == nil {
		panic(fmt.Sprintf("wiop: RegisterPublicInputs: nil cell for tag %q", tag))
	}

	if len(numericSuffix) > 0 {
		tag = suffixPublicInputTag(tag, numericSuffix[0])
	}

	if sys.LookupCell(cell.Context.ID) != cell {
		panic(fmt.Sprintf("wiop: RegisterPublicInputs: cell %q does not belong to this system", cell.Context.Path()))
	}

	if oldCell, pos := sys.LookupPublicInputByTag(tag); pos >= 0 {
		panic(fmt.Sprintf(
			"wiop: a public-input with this tag already exists while trying to "+
				"register a new cell as public-input with the same tag: "+
				"new-cell=%v, old-cell=%v, tag=%v",
			cell.Context.Path(), oldCell.Context.Path(), tag))
	}

	// The tag check above already caught a re-registration under the same tag, so
	// a tag still present here is a different one: either a re-tagging of an
	// already-registered cell or a hand-written annotation on the reserved key.
	// Both leave the cell's role ambiguous, so refuse instead of overwriting.
	if old, ok := PublicInputTagOf(cell); ok {
		panic(fmt.Sprintf(
			"wiop: RegisterPublicInputs: cell %q is already tagged %q (registering it as %q)",
			cell.Context.Path(), old, tag,
		))
	}

	// Backstop: registration always annotates, so a registered cell is caught by
	// the tag check above. This only fires if the annotation was removed behind
	// the API's back.
	if _, pos := sys.LookupPublicInput(cell.Context.ID); pos >= 0 {
		panic(fmt.Sprintf("wiop: RegisterPublicInputs: cell %q is already a public input", cell.Context.Path()))
	}

	cell.Annotations[PublicInputTagKey] = tag
	sys.PublicInputs = append(sys.PublicInputs, cell)
}

// PublicInputTagOf returns the tag [System.RegisterPublicInputs] attached to
// cell, and whether the cell carries one at all. A false second return means the
// cell was never registered as a public input.
//
// It panics if the reserved [PublicInputTagKey] holds something other than a
// [PublicInputTag]: only RegisterPublicInputs may write that key, so a foreign
// value there is a bug rather than a condition to handle.
func PublicInputTagOf(cell *Cell) (PublicInputTag, bool) {
	v, ok := cell.Annotations[PublicInputTagKey]
	if !ok {
		return "", false
	}

	tag, ok := v.(PublicInputTag)
	if !ok {
		panic(fmt.Sprintf(
			"wiop: cell %q has a non-tag value of type %T under the reserved key %q",
			cell.Context.Path(), v, PublicInputTagKey,
		))
	}

	return tag, true
}

// LookupPublicInput checks if a cell with the given object ID exists in the
// system as a public input and returns the corresponding cell. If the object ID
// is not matched against a public input cell, the function returns nil.
//
// This method additionally returns the position of the cell in the public-input
// vector (sys.PublicInputs) and -1 if the cell is not found.
func (sys *System) LookupPublicInput(objID ObjectID) (*Cell, int) {
	for i, cell := range sys.PublicInputs {
		if cell.Context.ID == objID {
			return sys.PublicInputs[i], i
		}
	}
	return nil, -1
}

// publicInputIndex maps each registered public-input cell's [ObjectID] to its
// position in the flat [PublicInput] wire format.
//
// [System.Prove] and [System.Verify] build it once and then test membership in
// O(1) while walking every cell of every round. Calling
// [System.LookupPublicInput] from those loops instead would make them
// O(cells x public-inputs), and both counts grow with the size of the protocol.
func (sys *System) publicInputIndex() map[ObjectID]int {
	idx := make(map[ObjectID]int, len(sys.PublicInputs))
	for i, cell := range sys.PublicInputs {
		idx[cell.Context.ID] = i
	}
	return idx
}

// suffixPublicInputTag appends n to tag as "_<n>". Both
// [System.RegisterPublicInputs] and [System.LookupPublicInputByTag] go through
// it so that a lookup always reconstructs exactly the tag that registration
// stored.
func suffixPublicInputTag(tag PublicInputTag, n int) PublicInputTag {
	return tag + publicInputTagSeparator + PublicInputTag(strconv.Itoa(n))
}

// LookupPublicInputByTag returns the public-input cell carrying tag, together
// with its position in sys.PublicInputs (and so in the flat [PublicInput]
// vector). It returns (nil, -1) when no public input carries that tag.
//
// numericSuffix is applied exactly as [System.RegisterPublicInputs] applies it:
// if given, the first element is appended to tag as "_<n>" and any further
// elements are ignored, so a cell registered under (Role, i) is found by
// (Role, i).
func (sys *System) LookupPublicInputByTag(tag PublicInputTag, numericSuffix ...int) (*Cell, int) {

	if len(numericSuffix) > 0 {
		tag = suffixPublicInputTag(tag, numericSuffix[0])
	}

	for i, cell := range sys.PublicInputs {
		if t, ok := PublicInputTagOf(cell); ok && t == tag {
			return sys.PublicInputs[i], i
		}
	}
	return nil, -1
}
