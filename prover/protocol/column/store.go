package column

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
)

// Store registers [Natural] for structs that can return the infos given a name
// and it is used by the [github.com/consensys/linea-monorepo/prover/protocol/wizard.ProverRuntime] and the
// [github.com/consensys/linea-monorepo/prover/protocol/wizard.VerifierRuntime] to store the columns. The store keeps
// tracks of the definition rounds of the columns and offers a handful of
// methods to resolve all the columns that have a particular status.
type Store struct {
	// indicesByNames allows to resolve the position of the info of a column by
	// supplying the name of the column.
	indicesByNames collection.Mapping[ifaces.ColID, columnPosition]
	// stores the columns informations by [round][posInRound]
	byRounds collection.VecVec[*storedColumnInfo]
}

// NewStore constructs an empty Store object
func NewStore() Store {
	return Store{
		indicesByNames: collection.NewMapping[ifaces.ColID, columnPosition](),
		byRounds:       collection.NewVecVec[*storedColumnInfo](),
	}
}

// columnPosition is a utility struct that stores the position of the commitment
// in Store.byRounds. It is used inside of the Natural so that they can
// track their own positions in the store.
type columnPosition struct {
	round      int
	posInRound int
}

// storedColumnInfo represents an entry in the [Store] and is also used as part
// of the [Natural] column.
type storedColumnInfo struct {
	// Size of the commitment
	Size int
	// ifaces.ColID of the column stored
	ID ifaces.ColID
	// Status of the commitment
	Status Status
	// IncludeInProverFS states the prover should include the column in his FS
	// transcript. This is used for columns that are recursed using
	// FullRecursion. This field is only meaningfull for [Ignored] columns as
	// they are excluded by default.
	IncludeInProverFS bool
}

// AddToRound constructs a [Natural], registers it in the [Store] and returns
// the column
//   - name must not be an empty string
//   - round must be provided
//   - name must not have been registered already
func (s *Store) AddToRound(round int, name ifaces.ColID, size int, status Status) ifaces.Column {

	if len(name) == 0 {
		utils.Panic("given an empty name")
	}

	if !utils.IsPowerOfTwo(size) {
		utils.Panic("can't register %v because it has a non-power of two size (%v)", name, size)
	}

	// Compute the location of the commitment in the store
	position := columnPosition{
		round:      round,
		posInRound: s.byRounds.LenOf(round),
	}

	// Constructing at the beginning does the validation early on
	nat := newNatural(name, position, s)
	infos := &storedColumnInfo{Size: size, ID: name, Status: status}

	// Panic if the entry already exist
	s.indicesByNames.InsertNew(name, position)
	s.byRounds.AppendToInner(round, infos)

	return nat
}

// GetSize returns the stored size of a [Natural] by its ID. This only works if
// the requested column is a [Natural].
func (s *Store) GetSize(n ifaces.ColID) int {
	if s == nil {
		panic("null pointer here")
	}
	info := s.info(n)
	return info.Size
}

// AllKeysAt returns the list of all keys for a given round. The result follows
// the insertion order of insertion) (=assignment order)
func (r *Store) AllKeysAt(round int) []ifaces.ColID {
	rnd := r.byRounds.MustGet(round)
	res := make([]ifaces.ColID, len(rnd))
	for i := range rnd {
		res[i] = rnd[i].ID
	}
	return res
}

// Returns the list of all the [ifaces.ColID] tagged with the [Committed] status so far
// at a given round. The order of the returned slice follows the insertion order.
func (r *Store) AllKeysCommittedAt(round int) []ifaces.ColID {
	rnd := r.byRounds.MustGet(round)
	res := make([]ifaces.ColID, 0, len(rnd))

	for i, info := range rnd {
		if info.Status != Committed {
			continue
		}

		res = append(res, rnd[i].ID)
	}
	return res
}

// AllHandleCommittedAt returns the list of all the [Committed] columns so far
// at a given round. The returned slice is ordered by order of insertion.
func (r *Store) AllHandleCommittedAt(round int) []ifaces.Column {
	rnd := r.byRounds.MustGet(round)
	res := make([]ifaces.Column, 0, len(rnd))

	for i, info := range rnd {
		if info.Status != Committed {
			continue
		}

		res = append(res, r.GetHandle(rnd[i].ID))
	}
	return res
}

// AllKeysIgnoredAt returns the list of all the [Ignored] columns ids so far at a
// given round. The returned slice is ordered by order of insertion.
func (r *Store) AllKeysIgnoredAt(round int) []ifaces.ColID {
	rnd := r.byRounds.MustGet(round)
	res := make([]ifaces.ColID, 0, len(rnd))

	for i, info := range rnd {
		if info.Status != Ignored {
			continue
		}

		res = append(res, rnd[i].ID)
	}
	return res
}

// AllKeysProof returns the list of all the [Proof] column's IDs ordered by
// round and then by order of insertion.
func (r *Store) AllKeysProof() []ifaces.ColID {
	res := []ifaces.ColID{}

	for round := 0; round < r.NumRounds(); round++ {
		proof := r.AllKeysProofAt(round)
		res = append(res, proof...)
	}

	return res
}

// AllKeysPublicInput returns the list of the [PublicInput] column's ID ordered
// by rounds and then by order ot insertion.
func (r *Store) AllKeysPublicInput() []ifaces.ColID {
	res := []ifaces.ColID{}

	for round := 0; round < r.NumRounds(); round++ {
		proof := r.AllKeysPublicInputAt(round)
		res = append(res, proof...)
	}

	return res
}

// AllKeysCommitted returns the list of all the IDs of the all the [Committed]
// columns ordered by rounds and then by IDs.
func (r *Store) AllKeysCommitted() []ifaces.ColID {
	res := []ifaces.ColID{}

	for round := 0; round < r.NumRounds(); round++ {
		for _, info := range r.byRounds.MustGet(round) {
			if info.Status != Committed {
				continue
			}
			res = append(res, info.ID)
		}
	}

	return res
}

// Returns the list of all the ignored messages so far
func (r *Store) AllKeysIgnored() []ifaces.ColID {
	res := []ifaces.ColID{}

	for round := 0; round < r.NumRounds(); round++ {
		for _, info := range r.byRounds.MustGet(round) {
			if info.Status != Ignored {
				continue
			}
			res = append(res, info.ID)
		}
	}

	return res
}

// AllKeysProofAt returns the list of all the IDs of the[Proof] messages at a
// given round. The returned list is ordered by order of insertion.
func (r *Store) AllKeysProofAt(round int) []ifaces.ColID {
	res := []ifaces.ColID{}
	rnd := r.byRounds.MustGet(round)

	for i, info := range rnd {
		if info.Status != Proof {
			continue
		}
		res = append(res, rnd[i].ID)
	}

	return res
}

// AllKeysPublicInputAt returns the list of all the prover messages in a given
// round. The resulting slice is ordered by order of insertion.
func (r *Store) AllKeysPublicInputAt(round int) []ifaces.ColID {
	res := []ifaces.ColID{}
	rnd := r.byRounds.MustGet(round)

	for i, info := range rnd {
		if info.Status != PublicInput {
			continue
		}
		res = append(res, rnd[i].ID)
	}

	return res
}

// Returns the list of all the [Precomputed] columns' ID. The returned slice is
// ordered by rounds and then by order of insertion.
func (r *Store) AllPrecomputed() []ifaces.ColID {
	res := []ifaces.ColID{}
	rnd := r.byRounds.MustGet(0) // precomputed are always at round zero

	for i, info := range rnd {
		if info.Status != Precomputed {
			continue
		}
		res = append(res, rnd[i].ID)
	}

	return res
}

// AllVerifyingKey returns the list of all the IDs of the [VerifyingKey] columns
// ordered by rounds and then by order of insertion.
func (r *Store) AllVerifyingKey() []ifaces.ColID {
	res := []ifaces.ColID{}
	rnd := r.byRounds.MustGet(0) // precomputed are always at round zero

	for i, info := range rnd {
		if info.Status != VerifyingKey {
			continue
		}
		res = append(res, rnd[i].ID)
	}

	return res
}

// Returns the status of a column by its ID. This will panic if the provided
// column is not registered in the store.
func (s *Store) Status(name ifaces.ColID) Status {
	return s.info(name).Status
}

// Change the status of a commitment
func (s *Store) SetStatus(name ifaces.ColID, status Status) {
	info := s.info(name)
	assertCorrectStatusTransition(info.Status, status)
	info.Status = status
}

// Get the info of a commitment by name, panic if not found
func (s *Store) info(name ifaces.ColID) *storedColumnInfo {
	pos := s.indicesByNames.MustGet(name)
	return s.byRounds.MustGet(pos.round)[pos.posInRound]
}

/*
Returns the list of all the keys ever. The result is returned in
Deterministic order.
*/
func (r *Store) AllKeys() []ifaces.ColID {
	res := []ifaces.ColID{}

	for roundID := 0; roundID < r.NumRounds(); roundID++ {
		ids := r.AllKeysAt(roundID)
		res = append(res, ids...)
	}

	return res
}

/*
Returns the number of rounds
*/
func (r *Store) NumRounds() int {
	return r.byRounds.Len()
}

/*
Make sure enough rounds are allocated up to the given length
No-op if enough rounds have been allocated, otherwise, will
reserve as many as necessary.
*/
func (r *Store) ReserveFor(newLen int) {
	if r.byRounds.Len() < newLen {
		r.byRounds.Reserve(newLen)
	}
}

/*
Returns all handle stores at a given round
*/
func (s *Store) AllHandlesAtRound(round int) []ifaces.Column {
	roundInfos := s.byRounds.MustGet(round)
	res := make([]ifaces.Column, len(roundInfos))
	for posInRound, info := range roundInfos {
		res[posInRound] = Natural{
			ID:       info.ID,
			position: columnPosition{round: round, posInRound: posInRound},
			store:    s,
		}
	}
	return res
}

/*
Returns all handle stores at a given round
*/
func (s *Store) AllHandlesAtRoundUnignored(round int) []ifaces.Column {
	roundInfos := s.byRounds.MustGet(round)
	res := make([]ifaces.Column, 0, len(roundInfos))

	for posInRound, info := range roundInfos {
		if info.Status == Ignored {
			continue
		}

		res = append(res, Natural{
			ID:       info.ID,
			position: columnPosition{round: round, posInRound: posInRound},
			store:    s,
		})
	}
	return res
}

/*
Returns the handle corresponding to a given name.
Panic if not found.
*/
func (s *Store) GetHandle(name ifaces.ColID) ifaces.Column {
	// Note that this panics if the entry is not present
	position := s.indicesByNames.MustGet(name)
	return Natural{
		ID:       name,
		position: position,
		store:    s,
	}
}

// Panics if the store does not have the name registered
func (s *Store) MustHaveName(name ifaces.ColID) {
	if !s.indicesByNames.Exists(name) {
		utils.Panic("don't have %v", name)
	}
}

// Panics if the commitment name is not registered or if
// the round is the wrong one
func (s *Store) MustBeInRound(name ifaces.ColID, round int) {

	if !s.indicesByNames.Exists() {
		utils.Panic("commitment %v not registered", name)
	}

	info := s.indicesByNames.MustGet(name)
	if info.round != round {
		utils.Panic("registered %v at round %v but asserted %v", name, info.round, round)
	}
}

// Returns if the `name` exist in the commitment store
func (s *Store) Exists(name ifaces.ColID) bool {
	return s.indicesByNames.Exists(name)
}

// Marks a commitment as ignored, this can happen during a
// vector slicing operation. Panics, if the name was not
// registered or if the commitment was already ignored.
func (s *Store) MarkAsIgnored(name ifaces.ColID) {
	infos := s.info(name)
	assertCorrectStatusTransition(infos.Status, Ignored)
	infos.Status = Ignored
}

// IsIgnored returns true if the passed column ID relates to a column bearing
// the [Ignored] status.
func (s *Store) IsIgnored(name ifaces.ColID) (ignored bool) {
	return s.Status(name) == Ignored
}

// Sanity-checks for the function changing the status of a column
func assertCorrectStatusTransition(old, new Status) {

	forbiddenTransition := false

	// Whitelist the ignoring whatever is the past status
	if new == Ignored {
		return
	}

	switch {
	// Verifying keys element are always computed offline no matter whats
	case old == VerifyingKey && new != VerifyingKey:
		forbiddenTransition = true
	// If it's ignored, it's ignored
	case old == Ignored && new != Ignored:
		forbiddenTransition = true
	// You can't change the status of the public inputs because that would
	// change the statement of the zkEVM.
	case old == PublicInput && new != PublicInput:
		forbiddenTransition = true
	// It's a special status and cannot be changed.
	case old == VerifierDefined && new != VerifierDefined:
		forbiddenTransition = true
	}

	if forbiddenTransition {
		utils.Panic("attempted the transition %v -> %v, which is forbidden", old.String(), new.String())
	}
}

// IgnoreButKeepInProverTranscript marks a column as ignored but also asks that
// the column stays included in the FS transcript. This is used as part of
// full-recursion where the commitments to an inner-proofs should not be sent to
// the verifier but should still play a part in the FS transcript.
func (s *Store) IgnoreButKeepInProverTranscript(colName ifaces.ColID) {
	in := s.info(colName)
	in.Status = Ignored
	in.IncludeInProverFS = true
}

// IsIgnoredAndNotKeptInTranscript indicates whether the column can be ignored
// from the transcript and is used during the Fiat-Shamir randomness generation.
func (s *Store) IsIgnoredAndNotKeptInTranscript(colName ifaces.ColID) bool {
	in := s.info(colName)
	return in.Status == Ignored && !in.IncludeInProverFS
}

// AllKeysProofsOrIgnoredButKeptInProverTranscript returns the list of the
// columns to be used as part of the FS transcript.
func (s *Store) AllKeysProofsOrIgnoredButKeptInProverTranscript(round int) []ifaces.ColID {
	res := []ifaces.ColID{}
	rnd := s.byRounds.MustGet(round) // precomputed are always at round zero

	for i, info := range rnd {

		ok := (info.Status == Proof) || (info.Status == Ignored && info.IncludeInProverFS)
		if !ok {
			continue
		}

		res = append(res, rnd[i].ID)
	}

	return res
}
