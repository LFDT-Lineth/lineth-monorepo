package column

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/google/uuid"
)

// PackedStore is a serialization-friendly intermediate struct used for
// serializing a column store.
type PackedStore [][]*storedColumnInfo

// Pack packs the store into a [PackedStore]
func (s *Store) Pack() PackedStore {
	res := make(PackedStore, s.byRounds.Len())
	for i := 0; i < s.byRounds.Len(); i++ {
		res[i] = s.byRounds.MustGet(i)
	}
	return res
}

func (p PackedStore) Unpack() *Store {
	store := NewStore()
	for rnd, arr := range p {
		for pir, info := range arr {
			store.byRounds.AppendToInner(rnd, info)
			store.indicesByNames.InsertNew(
				info.ID,
				columnPosition{
					round:      rnd,
					posInRound: pir,
				},
			)
		}
	}
	return store
}

// PackedNatural is serialization-friendly intermediate structure that is
// use to represent a natural column.
type PackedNatural struct {
	Store    *Store       `cbor:"s"`
	Round    int          `cbor:"r"`
	Position int          `cbor:"p"`
	ID       ifaces.ColID `cbor:"i"`
}

// Pack packs a [Natural] into a [PackedNatural]
func (nat Natural) Pack() PackedNatural {
	return PackedNatural{
		Store:    nat.store,
		Round:    nat.position.round,
		Position: nat.position.posInRound,
		ID:       nat.ID,
	}
}

func (unpacked PackedNatural) Unpack() Natural {
	return newNatural(
		unpacked.ID,
		columnPosition{round: unpacked.Round, posInRound: unpacked.Position},
		unpacked.Store,
	)
}

// PackedIdentifier returns an identifier that won't conflict with the
// serialization of a [PackedNatural].
func (nat Natural) UUID() uuid.UUID {
	return nat.store.info(nat.ID).uuid
}
