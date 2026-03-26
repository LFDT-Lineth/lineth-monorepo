package mock

import (
	"io"
	"math/big"
	"slices"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// AccountSegmentPattern (ASP) is an enum indicating the type of the account segment
// with precision.
type accountSegmentPattern int

const (
	// readOnlyAsp characterizes a segment relative to an account, that preexists
	// is never modified, deployed or deleted within the segment.
	readOnlyAsp accountSegmentPattern = iota + 1
	// readWrite characterizes a segment relative to a pre-existing account,
	// being modified but neither deployed or deleted.
	readWriteAsp
	// doesntExistAsp characterizes a segment touching an account that does not
	// exists and never exists in the frame of the segment. This can happen if
	// the BALANCE opcode is called targetting a non-existing account. The EVM
	// will return 0 in that case, but we still need to prove that the account
	// does not exists in the state.
	//
	// It also covers "ephemeral" segments: relative to an account that only
	// exists during the frame of the segment (the account is created and
	// deleted just after, before the blocks ends).
	missingOrTransient
	// createdAsp characterizes a segment touching an account that does not
	// exists at the beginning of the segment and exists at the end of the
	// segment. This is the case for an account initializationed contract or of a new account
	// receiving its first transfer.
	createdAsp
	// deletedAsp characterizes a segment in which the account exists at the
	// beginning but not at the end of the segment. This is the case for
	// a self-destructed account for instance.
	deletedAsp
	// redeployedAsp characterizes a segment in which the account pre-existed,
	// was deleted and then redeployed in the same block.
	redeployedAsp
)

// InitShomeiState initializes the state of a Shomei tree by applying the
// provided state. The shomei state is deterministically initialized. The
// accounts are initialized in alphabetical order of their addresses and the
// storage keys of each account are also initialized in alphabetical order.
func InitShomeiState(state State) *statemanager.WorldState {

	var (
		shomeiState = statemanager.NewWorldState()
		addresses   = sortedKeysOf(state)
	)

	for _, address := range addresses {

		var (
			acc         = state[address]
			storageTrie = statemanager.NewStorageTrie(address)
			stoKeys     = sortedKeysOf(acc.Storage)
		)

		for _, stoKey := range stoKeys {
			storageTrie.InsertAndProve(stoKey, acc.Storage[stoKey])
		}

		account := types.Account{
			Nonce:          acc.Nonce,
			Balance:        acc.Balance,
			StorageRoot:    storageTrie.TopRoot(),
			LineaCodeHash:  acc.LineaCodeHash,
			KeccakCodeHash: acc.KeccakCodeHash,
			CodeSize:       acc.CodeSize,
		}

		shomeiState.AccountTrie.InsertAndProve(address, account.WrappedForShomeiTraces())
		shomeiState.StorageTries.InsertNew(address, storageTrie)
	}

	return shomeiState
}

// StateLogsToShomeiTraces applies the state logs to the given shomei state and
// returns the associated Shomei traces. The state logs are squashed by accounts
// blocks and by storage keys.
func StateLogsToShomeiTraces(shomeiState *statemanager.WorldState, logs [][]StateAccessLog) [][]statemanager.DecodedTrace {
	res := make([][]statemanager.DecodedTrace, len(logs))
	for block := range res {
		accountSegments := splitInAccountSegment(logs[block])
		for _, accSeg := range accountSegments {
			traces := applyAccountSegmentToShomei(shomeiState, accSeg)
			res[block] = append(res[block], traces...)
		}
	}
	return res
}

// splitInAccountSegment reorders the logs by accounts preserving chronological
// order for logs relating to the same account. The function then returns a list
// of account segment. The chronological order within each account is important
// because this is what will allow us to understand the account access pattern
// at play with the account.
//
// The input is meant to be only the logs of a single block. This is because
// Shomei process all the blocks separately and independently.
func splitInAccountSegment(logs []StateAccessLog) [][]StateAccessLog {

	// To ensure the function does not have side-effects we work over a
	// deep-copy of the caller's parameters.
	logs = append([]StateAccessLog{}, logs...)

	// 1. Sort the logs by account using a stable sort. This will ensure
	// that all the account segments are continuous while preserving
	// chronological order within each account segment. The chronological
	// order is important because it will understand which pattern is at
	// play.
	slices.SortStableFunc(logs, func(x, y StateAccessLog) int {
		switch {
		case x.Address.Hex() < y.Address.Hex():
			return -1
		case x.Address.Hex() > y.Address.Hex():
			return 1
		}
		return 0
	})

	// 2. Isolate the account segments. E.g. logs that touch the same account
	accountSegments := [][]StateAccessLog{}
	for i := range logs {
		if i == 0 || logs[i].Address != logs[i-1].Address {
			accountSegments = append(accountSegments, []StateAccessLog{})
		}
		accountSegments[len(accountSegments)-1] = append(
			accountSegments[len(accountSegments)-1],
			logs[i],
		)
	}

	// 3. Reorder the account segment by hkey
	slices.SortFunc(accountSegments, func(a, b []StateAccessLog) int {
		a_ := poseidon2Hash(a[0].Address)
		b_ := poseidon2Hash(b[0].Address)
		return a_.Cmp(b_)
	})

	return accountSegments
}

// applyAccountSegmentToShomei applies the account segment to the current shomei
// state and returns the list of decoded traces.
func applyAccountSegmentToShomei(shomeiState *statemanager.WorldState, accSegment []StateAccessLog) []statemanager.DecodedTrace {

	var (
		initAccountValue = types.Account{}
		address          = accSegment[0].Address
		blockTrace       = []statemanager.DecodedTrace{}
	)

	if pos, found := shomeiState.AccountTrie.FindKey(address); found {
		initAccountValue = shomeiState.AccountTrie.Data.MustGet(pos).Value.Account
	}

	asp := identifyAccountSegment(initAccountValue, accSegment)
	subSegments := splitInSubSegmentsForShomei(accSegment)

	// Sanity-checking the fact that non-creating non-selfdestructing account
	// segments have only a single sub-segment.
	if (asp == readOnlyAsp || asp == readWriteAsp) && len(subSegments) != 1 {
		panic("non-creating non-selfdestructing account segments have only a single sub-segment")
	}

	switch asp {

	case readOnlyAsp:

		var (
			accPos, _          = shomeiState.AccountTrie.FindKey(address)
			initAcc            = shomeiState.AccountTrie.Data.MustGet(accPos).Value
			squashedAccSegment = squashSubSegmentForShomei(initAcc.Account, subSegments[0])
		)

		// Since the acount segment is read-only, we can assume that all the
		// the squashed logs are read-only
		sortByHKeyStable(squashedAccSegment)

		for _, log := range squashedAccSegment {
			switch {
			case log.Type == Storage:
				blockTrace = append(blockTrace, applySquashedStorageLog(shomeiState, log, normalLogAppMode))
			case log.Type == SquashedAccount:
				trace := shomeiState.AccountTrie.ReadNonZeroAndProve(address)
				return append(blockTrace, asDecodedTrace("0x", trace))
			}
		}

	case readWriteAsp:

		var (
			accPos, _          = shomeiState.AccountTrie.FindKey(address)
			initAcc            = shomeiState.AccountTrie.Data.MustGet(accPos).Value
			squashedAccSegment = squashSubSegmentForShomei(initAcc.Account, subSegments[0])
		)

		sortByHKeyStable(squashedAccSegment)
		sortByRWStable(squashedAccSegment)

		for _, log := range squashedAccSegment {
			switch {
			case log.Type == Storage:
				blockTrace = append(blockTrace, applySquashedStorageLog(shomeiState, log, normalLogAppMode))
			case log.Type == SquashedAccount:
				newAccount := log.Value.(types.Account)
				newStorageRoot := shomeiState.StorageTries.MustGet(address).TopRoot()
				newAccount.StorageRoot = newStorageRoot
				trace := shomeiState.AccountTrie.UpdateAndProve(address, newAccount.WrappedForShomeiTraces())
				return append(blockTrace, asDecodedTrace("0x", trace))
			}
		}

	case missingOrTransient:
		trace := shomeiState.AccountTrie.ReadZeroAndProve(address)
		return append(blockTrace, asDecodedTrace("0x", trace))
	}

	if asp == deletedAsp || asp == redeployedAsp {
		// The relevant storage segment is always the first one; assertedly
		// ending with a deletion in it.
		var (
			relevSubSegment    = subSegments[0]
			accPos, _          = shomeiState.AccountTrie.FindKey(address)
			initAcc            = shomeiState.AccountTrie.Data.MustGet(accPos).Value
			squashedAccSegment = squashSubSegmentForShomei(initAcc.Account, relevSubSegment)
		)

		// Since the storage traces will all be read-only due to the fact that
		// we ignore the "new" value since the account will be deleted, there
		// is no need to sort by RW
		sortByHKeyStable(squashedAccSegment)

		if relevSubSegment[len(relevSubSegment)-1].Type != AccountErasal {
			panic("expected deploy")
		}

		for _, log := range squashedAccSegment {
			switch {
			case log.Type == Storage:
				blockTrace = append(blockTrace, applySquashedStorageLog(shomeiState, log, ignorePosteriorLogAppMode))
			case log.Type == SquashedAccount:
				trace := shomeiState.AccountTrie.DeleteAndProve(address)
				blockTrace = append(blockTrace, asDecodedTrace("0x", trace))
				shomeiState.StorageTries.Del(address)
			}
		}
	}

	if asp == createdAsp || asp == redeployedAsp {
		// We need to insert the storage trie before we attempt to use the
		// function `applySquashedStorageLog` because it will attempt to access
		// the storage trie. And in this case, the storage trie does not
		// pre-exists
		storageTrie := statemanager.NewStorageTrie(address)
		shomeiState.StorageTries.InsertNew(address, storageTrie)

		// The relevant storage sub-segment is always the last one. Assertedly,
		// it starts with an account initialization log.
		var (
			relevSubSegment    = subSegments[len(subSegments)-1]
			squashedAccSegment = squashSubSegmentForShomei(types.Account{}, relevSubSegment)
		)

		sortByHKeyStable(squashedAccSegment)
		sortByRWStable(squashedAccSegment)

		if relevSubSegment[0].Type != AccountInit {
			panic("expected deploy")
		}

		for _, log := range squashedAccSegment {
			switch {
			case log.Type == Storage:
				blockTrace = append(blockTrace, applySquashedStorageLog(shomeiState, log, ignorePriorLogAppMode))
			case log.Type == SquashedAccount:
				newAccount := log.Value.(types.Account)
				newStorageRoot := shomeiState.StorageTries.MustGet(address).TopRoot()
				newAccount.StorageRoot = newStorageRoot
				trace := shomeiState.AccountTrie.InsertAndProve(address, newAccount.WrappedForShomeiTraces())
				blockTrace = append(blockTrace, asDecodedTrace("0x", trace))
			}
		}
	}

	return blockTrace
}

// identifyAccountSegment parses an account segment and returns the
// corresponding pattern of the segment. The function also performs a variety
// of sanity-checks to ensure the trace **can** be processed and is
// corresponding to something valid.
//
// The caller is expected to provide an initial value for the account being
// processed by the segment. For a non-existing account, the caller must provide
// the empty value `types.Account{}`
func identifyAccountSegment(accInitValue types.Account, accSegment []StateAccessLog) accountSegmentPattern {

	var (
		emptyInitAcc = accInitValue == types.Account{}
		hasDelete    = false
		hasDeploy    = false
		endsAsEmpty  = emptyInitAcc
		hasWrites    = false
	)

	for _, log := range accSegment {
		if log.Type == AccountInit {
			if !endsAsEmpty {
				panic("inconsistency: we deploy but the account already exists")
			}
			endsAsEmpty = false
			hasDeploy = true
		}

		if log.Type == AccountErasal {
			if endsAsEmpty {
				panic("inconsistency: we delete the account but it is already empty")
			}
			endsAsEmpty = true
			hasDelete = true
		}

		if log.IsWrite {
			hasWrites = true
		}
	}

	// The order of the cases is important and should be changed with caution
	switch {
	case emptyInitAcc && endsAsEmpty:
		return missingOrTransient
	case emptyInitAcc:
		return createdAsp
	case endsAsEmpty:
		return deletedAsp
	case hasDelete || hasDeploy:
		return redeployedAsp
	case hasWrites:
		return readWriteAsp
	default:
		return readOnlyAsp
	}
}

// splitInSubSegmentsForShomei takes a sequence of logs corresponding to an
// account segment and splits it into a list of sub-segment with the following
// rules:
//   - an account deletion always terminates at the end of a sub-segment
//   - an account deployment always starts a subsegment
func splitInSubSegmentsForShomei(accSegment []StateAccessLog) [][]StateAccessLog {

	subSegments := [][]StateAccessLog{{}}
	currSubSegmentId := 0

	for i, log := range accSegment {

		if len(subSegments[currSubSegmentId]) > 0 && log.Type == AccountInit {
			subSegments = append(subSegments, []StateAccessLog{})
			currSubSegmentId++
		}

		subSegments[currSubSegmentId] = append(subSegments[currSubSegmentId], log)

		if i < len(accSegment)-1 && log.Type == AccountErasal {
			subSegments = append(subSegments, []StateAccessLog{})
			currSubSegmentId++
		}
	}

	return subSegments
}

// squashSubSegmentForShomei squashes an array of logs. It assumes that the input logs
// are all relevant to the same account "sub-segment". The function does not
// mutate the input logs.
//
// This method will panic if it is passed an ephemeral sub-segment (one that
// starts with an account initialization and ends with a an account deletion).
func squashSubSegmentForShomei(initialAccountValue types.Account, logs []StateAccessLog) []StateAccessLog {

	logs = append([]StateAccessLog{}, logs...) // i.e. a deep copy of logs

	var (
		currBlock            = logs[0].Block
		accountAddress       = logs[0].Address
		startWithDeploy      = logs[0].Type == AccountInit
		endsWithSelfDestruct = logs[len(logs)-1].Type == AccountErasal
		squashedLogs         = []StateAccessLog{}
	)

	if startWithDeploy && endsWithSelfDestruct {
		panic("found an ephemeral trace")
	}

	// All the storage keys go first. Then, sort the storage keys by
	// alphabetical order. The goal is to regroup all the operations touching
	// the same storage slots
	slices.SortStableFunc(logs, func(a, b StateAccessLog) int {
		switch {
		case a.Type == Storage && b.Type != Storage:
			return -1
		case a.Type != Storage && b.Type == Storage:
			return 1
		case a.Type == Storage && b.Type == Storage && a.Key.Hex() < b.Key.Hex():
			return -1
		case a.Type == Storage && b.Type == Storage && a.Key.Hex() > b.Key.Hex():
			return 1
		default:
			return 0
		}
	})

	var (
		currStorageSlotKey         types.FullBytes32
		currStorageSlotInitalValue types.FullBytes32
		currAccountValue           = initialAccountValue
		accountWasUpdated          = false
	)

	// For each, log we squash the operation storage slot by storage slot and
	// then we compactify all the non-storage related fields into a single log.
	for i, log := range logs {

		// The rule to determine if the account touched by the subsegment should
		// be marked as a write is that any writing log in the sub-segment makes
		// the overall squashed account log a writing operation.
		accountWasUpdated = accountWasUpdated || log.IsWrite

		if log.Type == Storage {
			// Squashed storage log initialization: if the current log touches a
			// different storage slot as what is indicated in `currStorageSlotKey`,
			// then we started a new squashed log.
			if i == 0 || log.Key != currStorageSlotKey {
				currStorageSlotKey = log.Key
				if log.IsWrite {
					currStorageSlotInitalValue = log.OldValue.(types.FullBytes32)
				} else {
					currStorageSlotInitalValue = log.Value.(types.FullBytes32)
				}
			}

			// Squashed storage log finalization: if the next log is either
			// 	- OOB
			// 	- Not a storage log
			// 	- A storage log with a different key
			// Then, we close the current storage log
			if i == len(logs)-1 || logs[i+1].Type != Storage || logs[i+1].Key != currStorageSlotKey {
				currStorageSlotFinalValue := log.Value.(types.FullBytes32)
				newSquashedLog := StateAccessLog{
					Address: log.Address,
					Block:   log.Block,
					Type:    Storage,
					Key:     currStorageSlotKey,
					Value:   log.Value,
				}

				if currStorageSlotInitalValue != currStorageSlotFinalValue {
					newSquashedLog.IsWrite = true
					newSquashedLog.OldValue = currStorageSlotInitalValue
				}

				squashedLogs = append(squashedLogs, newSquashedLog)
			}
		}

		// Account-level access squashing: here we capture the writes operations
		// made on the account itself and apply them over the "currAccountValue"
		// which we will use after the loop to append the final "squashed account"
		// log.
		if log.Type != Storage && log.IsWrite {
			switch log.Type {
			case AccountInit:
				vals := log.Value.([]any)
				currAccountValue.CodeSize = vals[0].(int64)
				currAccountValue.KeccakCodeHash = vals[1].(types.FullBytes32)
				currAccountValue.LineaCodeHash = vals[2].(types.KoalaOctuplet)
				if currAccountValue.Balance == nil {
					// we give it a non-nil value because this is used to infer
					// the existence of the account in the `statesummary` module
					currAccountValue.Balance = big.NewInt(0)
				}
			case Balance:
				currAccountValue.Balance = log.Value.(*big.Int)
			case Nonce:
				currAccountValue.Nonce = log.Value.(int64)
			case AccountErasal:
				// Unnecessary to specify anything here because the new value
				// of the account will be disregarded anyway.
			}
		}
	}

	accountLevelLog := StateAccessLog{
		Address: accountAddress,
		Block:   currBlock,
		Type:    SquashedAccount,
		Value:   currAccountValue,
		IsWrite: accountWasUpdated,
	}

	squashedLogs = append(squashedLogs, accountLevelLog)
	return squashedLogs
}

// shomeiLogApplication specifies how the function [applySquashedLog] should
// interpret a log. The zero value corresponds to the normal mode
type shomeiLogApplicationMode int

const (
	// normalLogAppMode corresponds to a litteral application of the log
	normalLogAppMode shomeiLogApplicationMode = iota
	// ignorePriorLogAppMode tells to convert reads and deletion into
	// readZeroes, updates are turned into insertion.
	ignorePriorLogAppMode
	// ignorePosterior tells to convert every write operation into a read
	// operation where only the initial value is read.
	ignorePosteriorLogAppMode
)

// applySquashedLog applies a "squashed" access Storage log (which are obtained
// through the [squashLogsForShomei]) function (meaning the function expects
// only logs of type Storage). The function also admit an
// extra parameter `mode` which specifies how the log has to be interpreted.
func applySquashedStorageLog(
	shomeiState *statemanager.WorldState,
	log StateAccessLog,
	mode shomeiLogApplicationMode,
) statemanager.DecodedTrace {

	if log.Type != Storage {
		panic("expected only s")
	}

	switch {
	case mode == ignorePosteriorLogAppMode && log.IsWrite:
		log.IsWrite = false
		log.Value = log.OldValue
		log.OldValue = nil
	case mode == ignorePriorLogAppMode && log.IsWrite:
		log.OldValue = types.FullBytes32{}
	case mode == ignorePriorLogAppMode:
		log.Value = types.FullBytes32{}
	}

	// ensure the invariant that no "squashed" write operation writes the same
	// value as the original value: otherwise it should be read.
	if log.Value == log.OldValue {
		log.IsWrite = false
		log.OldValue = nil
	}

	var (
		valueIsEmpty = log.Value == types.FullBytes32{}
		oldIsEmpty   = log.OldValue == types.FullBytes32{}
		address      = log.Address
		storageTrie  = shomeiState.StorageTries.MustGet(address)
	)

	switch {
	case log.IsWrite && valueIsEmpty && !oldIsEmpty:
		return asDecodedTrace(address.Hex(), storageTrie.DeleteAndProve(log.Key))
	case log.IsWrite && !valueIsEmpty && oldIsEmpty:
		return asDecodedTrace(address.Hex(), storageTrie.InsertAndProve(log.Key, log.Value.(types.FullBytes32)))
	case log.IsWrite && !valueIsEmpty && !oldIsEmpty:
		return asDecodedTrace(address.Hex(), storageTrie.UpdateAndProve(log.Key, log.Value.(types.FullBytes32)))
	case !log.IsWrite && valueIsEmpty:
		return asDecodedTrace(address.Hex(), storageTrie.ReadZeroAndProve(log.Key))
	case !log.IsWrite && !valueIsEmpty:
		return asDecodedTrace(address.Hex(), storageTrie.ReadNonZeroAndProve(log.Key))
	default:
		panic("illegal case")
	}
}

// sortAddressesOf returns a deterministically ordered list of addresses that
// are keys of ls.
func sortedKeysOf[T sortable, U any](m map[T]U) []T {
	res := make([]T, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	slices.SortStableFunc(res, func(x, y T) int {
		switch {
		case x.Hex() < y.Hex():
			return -1
		case y.Hex() > x.Hex():
			return 1
		}
		return 0
	})
	return res
}

// asDecodedTrace converts a shomei trace
func asDecodedTrace(location string, trace accumulator.Trace) statemanager.DecodedTrace {

	var new = statemanager.DecodedTrace{Underlying: trace}
	switch trace.(type) {
	case statemanager.ReadZeroTraceST, statemanager.ReadZeroTraceWS:
		new.Type = statemanager.READ_ZERO_TRACE_CODE
	case statemanager.ReadNonZeroTraceST, statemanager.ReadNonZeroTraceWS:
		new.Type = statemanager.READ_TRACE_CODE
	case statemanager.InsertionTraceST, statemanager.InsertionTraceWS:
		new.Type = statemanager.INSERTION_TRACE_CODE
	case statemanager.UpdateTraceST, statemanager.UpdateTraceWS:
		new.Type = statemanager.UPDATE_TRACE_CODE
	case statemanager.DeletionTraceST, statemanager.DeletionTraceWS:
		new.Type = statemanager.DELETION_TRACE_CODE
	default:
		utils.Panic("invalid type: %T", trace)
	}

	switch trace.(type) {
	case statemanager.ReadNonZeroTraceST, statemanager.ReadZeroTraceST, statemanager.InsertionTraceST, statemanager.UpdateTraceST, statemanager.DeletionTraceST:
		if len(location) == 2 {
			panic("storage trie operation but the location was 0x")
		}
		new.Location = location
	default:
		new.Location = "0x"
	}

	return new
}

// sortable is an ad-hoc interface type used by [sortedKeysOf]
type sortable interface {
	Hex() string
	comparable
}

// AssertShomeiAgree obtains frames from a StateLogBuilder which was run on an initial state
// It then generates corresponding shomei traces from the frames using StateLogsToShomeiTraces
// it uses statemanager.CheckTraces to obtain a sequence of root hashes and checks whether the
// first root hash is corresponds to the hash of the initial state
func AssertShomeiAgree(t *testing.T, state State, traces [][]StateAccessLog) {
	var (
		shomeiState  = InitShomeiState(state)
		initRootHash = shomeiState.AccountTrie.TopRoot()
		shomeiTraces = StateLogsToShomeiTraces(shomeiState, traces)
	)

	for _, blockTraces := range shomeiTraces {

		old, new, err := statemanager.CheckTraces(blockTraces)
		if err != nil {
			t.Fatalf("trace verification failed: %v", err.Error())
		}

		if old != initRootHash {
			t.Fatalf("state root hash mismatch")
		}

		initRootHash = new
	}
}

func poseidon2Hash(m io.WriterTo) types.KoalaOctuplet {
	h := poseidon2.NewMDHasher()
	m.WriteTo(h)
	d := h.Sum(nil)
	return types.MustBytesToKoalaOctuplet(d)
}

func sortByHKeyStable(subSegment []StateAccessLog) {
	slices.SortStableFunc(
		subSegment[:len(subSegment)-1], // The last entry will be the account-level log which we don't want to sort
		func(a, b StateAccessLog) int {
			switch {
			case poseidon2Hash(a.Key).Hex() < poseidon2Hash(b.Key).Hex():
				return -1
			case poseidon2Hash(a.Key).Hex() > poseidon2Hash(b.Key).Hex():
				return 1
			default:
				return 0
			}
		},
	)
}

func sortByRWStable(subSegment []StateAccessLog) {
	slices.SortStableFunc(
		subSegment[:len(subSegment)-1], // The last entry will be the account-level log which we don't want to sort
		func(a, b StateAccessLog) int {
			if a.IsWrite == b.IsWrite {
				return 0
			}

			if a.IsWrite {
				return 1
			}

			return -1
		},
	)
}
