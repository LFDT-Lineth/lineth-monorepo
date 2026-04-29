package statemanager

import (
	"encoding/json"
	"fmt"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Handy type aliases
type (
	Digest      = types.KoalaOctuplet
	Address     = types.EthAddress
	Account     = types.AccountShomeiTraces
	FullBytes32 = types.FullBytes32
)

type (
	// Aliases for the account tree
	AccountTrie = accumulator.ProverState[Address, Account]
	StorageTrie = accumulator.ProverState[FullBytes32, FullBytes32]

	// Account VS
	AccountVerifier = accumulator.VerifierState[Address, Account]
	StorageVerifier = accumulator.VerifierState[FullBytes32, FullBytes32]

	// ReadNonZeroTrace
	ReadNonZeroTraceWS = accumulator.ReadNonZeroTrace[Address, Account]
	ReadNonZeroTraceST = accumulator.ReadNonZeroTrace[FullBytes32, FullBytes32]

	// ReadZeroTrace
	ReadZeroTraceWS = accumulator.ReadZeroTrace[Address, Account]
	ReadZeroTraceST = accumulator.ReadZeroTrace[FullBytes32, FullBytes32]

	// InsertionTrace
	InsertionTraceWS = accumulator.InsertionTrace[Address, Account]
	InsertionTraceST = accumulator.InsertionTrace[FullBytes32, FullBytes32]

	// UpdateTrace
	UpdateTraceWS = accumulator.UpdateTrace[Address, Account]
	UpdateTraceST = accumulator.UpdateTrace[FullBytes32, FullBytes32]

	// DeletionTrace
	DeletionTraceWS = accumulator.DeletionTrace[Address, Account]
	DeletionTraceST = accumulator.DeletionTrace[FullBytes32, FullBytes32]
)

// Code to identify the type of an update
const (
	READ_TRACE_CODE      int = 0
	READ_ZERO_TRACE_CODE int = 1
	INSERTION_TRACE_CODE int = 2
	UPDATE_TRACE_CODE    int = 3
	DELETION_TRACE_CODE  int = 4
)

// Represents a Shomei output
type ShomeiOutput struct {
	Result struct {
		ZkParentStateRootHash types.KoalaOctuplet `json:"zkParentStateRootHash"`
		ZkStateMerkleProof    [][]DecodedTrace    `json:"zkStateMerkleProof"`
	} `json:"result"`
}

// When decoding a trace, before we can infer its type. We need to first read
// its location and its type
type DecodedTrace struct {
	Location string `json:"location"`
	Type     int    `json:"type"`
	// Can be any type of trace in the 5 possible types, for the either the
	// world-state or an storage-trie.
	Underlying accumulator.Trace
	// decides whether the trace should be skipped
	IsSkipped bool
}

func (dec *DecodedTrace) UnmarshalJSON(data []byte) error {

	if string(data) == "null" {
		return nil
	}

	// First decode using reflection-only. The goal is to determine whether
	// the location corresponds to the account trie or a storage trie and to
	// determine the type of the trace to deserialize.
	var protoDec struct {
		Location string `json:"location"`
		Type     int    `json:"type"`
	}
	if err := json.Unmarshal(data, &protoDec); err != nil {
		return fmt.Errorf("unmarshalling state-manager trace : could not access location and type of trace %v: error: %v", string(data), err)
	}
	dec.Location = protoDec.Location
	dec.Type = protoDec.Type

	// This indicates the the location field is missing. Unfortunately, we cannot
	// easily determine if the key "type" is missing or not given that go will
	// return `0` which is a valid type.
	if len(dec.Location) == 0 {
		return fmt.Errorf("unmarshalling state-manager trace : key `location` is missing from %v", string(data))
	}

	isWorldState := dec.Location == "0x"

	switch {
	case dec.Type == 0 && !isWorldState:
		underlying := ReadNonZeroTraceST{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal ReadNonZero trace `%v` for the storage trie : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 1 && !isWorldState:
		underlying := ReadZeroTraceST{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal ReadZero trace `%v` for the storage trie : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 2 && !isWorldState:
		underlying := InsertionTraceST{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal Insertion trace `%v` for the storage trie : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 3 && !isWorldState:
		underlying := UpdateTraceST{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal Update trace `%v` for the storage trie : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 4 && !isWorldState:
		underlying := DeletionTraceST{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal Deletion trace `%v` for the storage trie : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 0 && isWorldState:
		underlying := ReadNonZeroTraceWS{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal ReadNonZero trace `%v` for the world state : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 1 && isWorldState:
		underlying := ReadZeroTraceWS{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal ReadZero trace `%v` for the world state : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 2 && isWorldState:
		underlying := InsertionTraceWS{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal Insertion trace `%v` for the world state : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 3 && isWorldState:
		underlying := UpdateTraceWS{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal Update trace `%v` for the world state : %w", string(data), err)
		}
		dec.Underlying = underlying

	case dec.Type == 4 && isWorldState:
		underlying := DeletionTraceWS{}
		if err := json.Unmarshal(data, &underlying); err != nil {
			return fmt.Errorf("unmarshalling state-manager trace : could not unmarshal Deletion trace `%v` for the world state : %w", string(data), err)
		}
		dec.Underlying = underlying

	default:
		return fmt.Errorf("traces does not map to any known state trace: location=%v, type=%v", dec.Location, dec.Type)
	}

	return nil
}

func (dec DecodedTrace) MarshalJSON() ([]byte, error) {
	return json.Marshal(dec.Underlying)
}

func (dec *DecodedTrace) isWorldState() bool {
	return dec.Location == WS_LOCATION
}
