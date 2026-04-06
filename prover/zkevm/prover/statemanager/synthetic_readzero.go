package statemanager

import (
	"encoding/binary"
	"log"
	"slices"
	"strings"

	execstatemanager "github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// syntheticEntry is an SCP storage entry that needs a synthetic ReadZero trace.
type syntheticEntry struct {
	address    types.EthAddress
	storageKey types.FullBytes32
	block      int
}

// injectSyntheticReadZeroTraces injects synthetic ReadZero traces for
// transient 0→0 storage writes in insertion segments.
func (sm *StateManager) injectSyntheticReadZeroTraces(run *wizard.ProverRuntime, shomeiTraces *[][]execstatemanager.DecodedTrace) {
	if sm.StateSummary.ArithmetizationLink == nil {
		return
	}
	scp := &sm.StateSummary.ArithmetizationLink.Scp

	entries := detectTransientInsertionEntries(run, scp)
	if len(entries) == 0 {
		return
	}

	type addrBlock struct {
		address types.EthAddress
		block   int
	}
	grouped := map[addrBlock][]types.FullBytes32{}
	for _, e := range entries {
		key := addrBlock{address: e.address, block: e.block}
		grouped[key] = append(grouped[key], e.storageKey)
	}

	for ab, keys := range grouped {
		blockIdx := ab.block - 1
		if blockIdx < 0 || blockIdx >= len(*shomeiTraces) {
			continue
		}

		emptyTrie := execstatemanager.NewStorageTrie(ab.address)
		var syntheticTraces []execstatemanager.DecodedTrace
		for _, storageKey := range keys {
			readZeroTrace := emptyTrie.ReadZeroAndProve(storageKey)
			syntheticTraces = append(syntheticTraces, execstatemanager.DecodedTrace{
				Location:   ab.address.Hex(),
				Type:       execstatemanager.READ_ZERO_TRACE_CODE,
				Underlying: readZeroTrace,
			})
		}

		blockTraces := (*shomeiTraces)[blockIdx]
		insertIdx := findStorageInsertionPoint(blockTraces, ab.address)
		if insertIdx < 0 {
			continue
		}

		log.Printf("synthetic_readzero: injecting %d synthetic ReadZero traces for address=%s block=%d at trace index %d",
			len(syntheticTraces), ab.address.Hex(), ab.block, insertIdx)

		(*shomeiTraces)[blockIdx] = slices.Insert(blockTraces, insertIdx, syntheticTraces...)
	}
}

// findStorageInsertionPoint returns where to inject synthetic storage traces
// for the given account. Returns -1 if not found.
func findStorageInsertionPoint(blockTraces []execstatemanager.DecodedTrace, targetAddr types.EthAddress) int {
	targetHex := targetAddr.Hex()
	for i, trace := range blockTraces {
		if trace.Location == execstatemanager.WS_LOCATION {
			continue
		}
		if strings.EqualFold(trace.Location, targetHex) {
			return i
		}
	}
	for i, trace := range blockTraces {
		if trace.Location != execstatemanager.WS_LOCATION {
			continue
		}
		addr, err := trace.GetRelatedAccount()
		if err != nil {
			continue
		}
		if addr == targetAddr {
			return i + 1
		}
	}
	return -1
}

// detectTransientInsertionEntries finds insertion segments where valueNext=0
// on the last KOC row but some value is non-zero within the segment.
func detectTransientInsertionEntries(run *wizard.ProverRuntime, scp *statesummary.HubColumnSet) []syntheticEntry {
	var entries []syntheticEntry
	lastSegmentStart := 0

	for index := 0; index < scp.PeekAtStorage.Size(); index++ {
		isStorage := scp.PeekAtStorage.GetColAssignmentAt(run, index)
		if !isStorage.IsOne() {
			continue
		}

		firstKOC := scp.FirstKOCBlock.GetColAssignmentAt(run, index)
		if firstKOC.IsOne() {
			lastSegmentStart = index
		}

		lastKOC := scp.LastKOCBlock.GetColAssignmentAt(run, index)
		if !lastKOC.IsOne() {
			continue
		}

		existsStart := scp.ExistsFirstInBlock.GetColAssignmentAt(run, lastSegmentStart)
		if !existsStart.IsZero() {
			continue
		}
		existsEnd := scp.ExistsFinalInBlock.GetColAssignmentAt(run, index)
		if !existsEnd.IsOne() {
			continue
		}

		if !allLimbsZero(run, scp.ValueHINext[:], index) || !allLimbsZero(run, scp.ValueLONext[:], index) {
			continue
		}

		hasNonZero := false
		for j := lastSegmentStart; j <= index; j++ {
			for i := range common.NbLimbU128 {
				vchi := scp.ValueHICurr[i].GetColAssignmentAt(run, j)
				vclo := scp.ValueLOCurr[i].GetColAssignmentAt(run, j)
				vnhi := scp.ValueHINext[i].GetColAssignmentAt(run, j)
				vnlo := scp.ValueLONext[i].GetColAssignmentAt(run, j)
				if !vchi.IsZero() || !vclo.IsZero() || !vnhi.IsZero() || !vnlo.IsZero() {
					hasNonZero = true
					break
				}
			}
			if hasNonZero {
				break
			}
		}
		if !hasNonZero {
			continue
		}

		addr := extractEthAddressFromScp(run, scp, index)
		key := extractStorageKeyFromScp(run, scp, index)
		block := extractBlockNumberFromScp(run, scp, index)

		log.Printf("synthetic_readzero: detected transient insertion entry at SCP index %d: address=%s key=%s block=%d",
			index, addr.Hex(), key.Hex(), block)

		entries = append(entries, syntheticEntry{
			address:    addr,
			storageKey: key,
			block:      block,
		})
	}
	return entries
}

// allLimbsZero returns true if all limb columns are zero at index.
func allLimbsZero(run *wizard.ProverRuntime, cols []ifaces.Column, index int) bool {
	for _, col := range cols {
		v := col.GetColAssignmentAt(run, index)
		if !v.IsZero() {
			return false
		}
	}
	return true
}

// extractEthAddressFromScp reconstructs an address from SCP AddressHI/LO limbs.
func extractEthAddressFromScp(run *wizard.ProverRuntime, scp *statesummary.HubColumnSet, index int) types.EthAddress {
	var addr [20]byte
	offset := 0
	for i := range common.NbLimbU32 {
		v := scp.AddressHI[i].GetColAssignmentAt(run, index)
		b := v.Bytes()
		addr[offset] = b[2]
		addr[offset+1] = b[3]
		offset += 2
	}
	for i := range common.NbLimbU128 {
		v := scp.AddressLO[i].GetColAssignmentAt(run, index)
		b := v.Bytes()
		addr[offset] = b[2]
		addr[offset+1] = b[3]
		offset += 2
	}
	return types.EthAddress(addr)
}

// extractStorageKeyFromScp reconstructs a storage key from SCP KeyHI/LO limbs.
func extractStorageKeyFromScp(run *wizard.ProverRuntime, scp *statesummary.HubColumnSet, index int) types.FullBytes32 {
	var key [32]byte
	offset := 0
	for i := range common.NbLimbU128 {
		v := scp.KeyHI[i].GetColAssignmentAt(run, index)
		b := v.Bytes()
		key[offset] = b[2]
		key[offset+1] = b[3]
		offset += 2
	}
	for i := range common.NbLimbU128 {
		v := scp.KeyLO[i].GetColAssignmentAt(run, index)
		b := v.Bytes()
		key[offset] = b[2]
		key[offset+1] = b[3]
		offset += 2
	}
	return types.FullBytes32(key)
}

// extractBlockNumberFromScp extracts the block number from SCP BlockNumber limbs.
func extractBlockNumberFromScp(run *wizard.ProverRuntime, scp *statesummary.HubColumnSet, index int) int {
	var blockBytes [8]byte
	for i := range common.NbLimbU64 {
		v := scp.BlockNumber[i].GetColAssignmentAt(run, index)
		b := v.Bytes()
		blockBytes[i*2] = b[2]
		blockBytes[i*2+1] = b[3]
	}
	return utils.ToInt(binary.BigEndian.Uint64(blockBytes[:]))
}
