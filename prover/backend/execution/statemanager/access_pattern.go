package statemanager

import (
	"errors"
	"fmt"
)

/*
For a given account, the trace can only have the following patterns. The regular expressions are given below:
| — denotes an OR
patterns we keep track of in the Shomei for state consistency check with the arithmetization
each segment corresponds to the delta for one account for one block
each segment corresponds to the delta of the block has only one of the following:

Non-existing account: READ_ZERO_WS
Account creation: (INSERT_ST | READ_ZERO_ST)* INSERT_WS
Account deleted: (READ_ZERO_ST | READ_NON_ZERO_ST)* DELETE_WS
Regular access with write: (ANY_ST)* UPDATE_WS
Regular access read-only: (READ_NON_ZERO_ST | READ_ZERO_ST)* READ_NON_ZERO_WS
Account redeployed: (READ_ZERO_ST | READ_NON_ZERO_ST)* DELETE_WS concatenated (INSERT_ST | READ_ZERO_ST)* INSERT_WS
*/

func inspectPattern(traces []DecodedTrace) (err error) {

	matches := []func(traces []DecodedTrace) (bool, error){
		isMissingAccRead,
		isAccCreation,
		isAccDeletion,
		isAccRead,
		isAccUpdate,
		isAccRedeploy,
	}

	for _, m := range matches {
		ok, err := m(traces)
		if err != nil {
			return err
		}
		// found a match, we can return
		if ok {
			return nil
		}
	}

	// No match was found : return an error with a list of the types
	ts := []string{}
	for i := range traces {
		ts = append(ts, fmt.Sprintf("%T", traces[i].Underlying))
	}
	return fmt.Errorf("no match found : %v", ts)
}

// returns true if there is a missing account reading.
// Error, if the trace length is not one.
func isMissingAccRead(traces []DecodedTrace) (ok bool, err error) {

	// Whitelist the pattern : [READ_ZERO_WS]
	if _, ok := traces[0].Underlying.(ReadZeroTraceWS); ok && len(traces) == 1 {
		return true, nil
	}

	// Look for errors
	for _, trace := range traces {
		if _, ok := trace.Underlying.(ReadZeroTraceWS); ok {
			return false, fmt.Errorf("found read zero in a trace whose length is larger than 1")
		}
	}

	// Otherwise, it is just a mismatch
	return false, nil
}

// return true if the traces contains only a single INSERT_WS
// at the end error if true and the ST storage are inconsistents
func isAccCreation(traces []DecodedTrace) (ok bool, err error) {
	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].Underlying.(InsertionTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {
		trace := traces[i]
		// If there is another, then it may be a redeployment
		if trace.isWorldState() {
			return false, nil
		}

		// Also attempt to cast the trace as a
		switch t := trace.Underlying.(type) {
		case ReadZeroTraceST, InsertionTraceST:
			// PASS: these are whitelisted operations
		default:
			// Note: at this point we do not know if the error is actual
			// we must return the error outside of the loop. We only keep
			// the first error.
			if err != nil {
				err = errors.Join(
					err,
					fmt.Errorf("invalid trace : found %T in an insertion trace", t),
				)
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	if err = checkStorageTraceOrder(traces[:len(traces)-1]); err != nil {
		return false, fmt.Errorf("in account-creation segment: %w", err)
	}

	return true, nil
}

// return true if the traces contains only a single INSERT_WS at the end
// error if true and the ST storage are inconsistents
func isAccDeletion(traces []DecodedTrace) (ok bool, err error) {
	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].Underlying.(DeletionTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {
		trace := traces[i]
		if trace.isWorldState() {
			return false, fmt.Errorf("invalid deletion preceded by another trace %++v", trace)
		}

		// Also attempt to cast the trace as a
		switch t := trace.Underlying.(type) {
		case ReadNonZeroTraceST, ReadZeroTraceST:
			// PASS: these are whitelisted operations
		default:
			// Note: at this point we do not know if the error is actual
			// we must return the error outside of the loop. We only keep
			// the first error.
			if err != nil {
				err = fmt.Errorf("invalid trace : found %T in an deletion trace", t)
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	if err = checkStorageTraceOrder(traces[:len(traces)-1]); err != nil {
		return false, fmt.Errorf("in account-deletion segment: %w", err)
	}

	return true, nil
}

// return true if the traces contains only a single UPDATE_WS at the end
// error if true and the ST storage are inconsistents
func isAccUpdate(traces []DecodedTrace) (ok bool, err error) {
	// First check that the last entry is an update of the WS
	_, ok = traces[len(traces)-1].Underlying.(UpdateTraceWS)
	if !ok {
		return false, nil
	}

	// edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {
		trace := traces[i]
		if trace.isWorldState() {
			return false, fmt.Errorf("invalid update preceded by another trace %++v", trace)
		}

		// All ST traces are allowed
	}

	if err = checkStorageTraceOrder(traces[:len(traces)-1]); err != nil {
		return false, fmt.Errorf("in account-update segment: %w", err)
	}

	return true, nil
}

// return true if the traces contains only a single UPDATE_WS at the end
// error if true and the ST storage are inconsistents
func isAccRead(traces []DecodedTrace) (ok bool, err error) {
	// First check that the first or last entry is an insertion
	// For some reason, Shomei will place the account read in the first position
	// while the specification says to do it
	_, okFirst := traces[0].Underlying.(ReadNonZeroTraceWS)
	_, okLast := traces[len(traces)-1].Underlying.(ReadNonZeroTraceWS)
	if !okFirst && !okLast {
		return false, nil
	}

	if okFirst && okLast && len(traces) > 1 {
		return false, errors.New("the segment contains two ACCOUNT_READ")
	}

	// Edge-case : the account is read but its storage isn't
	if len(traces) == 1 {
		return true, nil
	}

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces); i++ {

		if i == 0 && okFirst {
			continue
		}

		if i == len(traces)-1 && okLast {
			continue
		}

		trace := traces[i]
		if trace.isWorldState() {
			return false, fmt.Errorf("invalid read preceded by another trace %++v", trace)
		}

		// Test the type of the past traces: since we are doing a read operation,
		// the segment may only read the storage.
		switch t := trace.Underlying.(type) {
		case ReadNonZeroTraceST, ReadZeroTraceST:
			// PASS: these are whitelisted operations
		default:
			// Note: at this point we do not know if the error is actual
			// we must return the error outside of the loop. We only keep
			// the first error.
			if err != nil {
				err = fmt.Errorf("invalid trace : found %T in an account read trace", t)
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	// If the length is more than one, place the account-level trace at the end
	if okFirst && len(traces) > 1 {
		accTrace := traces[0]
		copy(traces[:], traces[1:])
		traces[len(traces)-1] = accTrace
	}

	if err = checkStorageTraceOrder(traces[:len(traces)-1]); err != nil {
		return false, fmt.Errorf("in acc-read segment: %w", err)
	}

	return true, nil
}

func isAccRedeploy(traces []DecodedTrace) (ok bool, err error) {

	// First check that the last entry is an insertion
	_, ok = traces[len(traces)-1].Underlying.(InsertionTraceWS)
	if !ok {
		return false, nil
	}

	// Edge-case : insertion without touching the empty tree
	if len(traces) == 1 {
		return false, nil
	}

	var (
		foundDeletion bool
		posDeletion   int
	)

	// Then, wheck that there are no more ws_traces in the list
	for i := 0; i < len(traces)-1; i++ {

		// Check if the current trace is a deletion
		_, isDeletion := traces[i].Underlying.(DeletionTraceWS)

		// Check that there cannot be two deletion
		if isDeletion && foundDeletion {
			return false, fmt.Errorf("found more than two deletion")
		}

		// Else set the foundDeletionFlag to true
		if isDeletion {
			foundDeletion = isDeletion
			posDeletion = i
			continue
		}

		// If the trace is still WS type its invalid
		if traces[i].isWorldState() {
			return false, fmt.Errorf("found a %v in a trace ending with an insertion", traces[i])
		}

		// Then the trace must be of type READ
		if !foundDeletion {
			switch t := traces[i].Underlying.(type) {
			case ReadZeroTraceST, ReadNonZeroTraceST:
				// PASS
			default:
				// Note: at this point we do not know if the error is actual
				// we must return the error outside of the loop. We only keep
				// the first error.
				if err != nil {
					err = errors.Join(
						err,
						fmt.Errorf("invalid trace : found %T in an deletion trace", t),
					)
				}
			}
		}

		if foundDeletion {
			switch t := traces[i].Underlying.(type) {
			case ReadZeroTraceST, InsertionTraceST:
				// PASS
			default:
				// Note: at this point we do not know if the error is actual
				// we must return the error outside of the loop. We only keep
				// the first error.
				if err != nil {
					err = errors.Join(
						err,
						fmt.Errorf("invalid trace : found %T in an deletion trace", t),
					)
				}
			}
		}
	}

	// Now, look for error. The only acceptable
	if err != nil {
		return false, err
	}

	err = errors.Join(
		checkStorageTraceOrder(traces[:posDeletion]),
		checkStorageTraceOrder(traces[posDeletion+1:len(traces)-1]),
	)

	if err != nil {
		return false, fmt.Errorf("in redeploy acc segment: %w", err)
	}

	return true, nil
}

func checkStorageTraceOrder(traces []DecodedTrace) (err error) {

	// Check that the storage slots are sorted in the correct order (by R/W then
	// by Hkey).
	for i := 0; i < len(traces)-2; i++ {

		var (
			curr     = traces[i].Underlying
			next     = traces[i+1].Underlying
			currRW   = curr.RWInt()
			nextRW   = next.RWInt()
			currHKey = curr.HKey()
			nextHKey = next.HKey()
		)

		if currRW > nextRW {
			err = errors.Join(
				err,
				fmt.Errorf("trace %v is a write but followed by a read", i),
			)
		}

		if currRW == nextRW && currHKey.Cmp(nextHKey) >= 0 {
			fmt.Printf("currRW=%v nextRW=%v currT=%T nextT=%T\n", currRW, nextRW, curr, next)
			err = errors.Join(
				err,
				fmt.Errorf("storage trace %v has an HKey larger or equal than the next one `%x` >= `%x`", i, currHKey, nextHKey),
			)
			panic("boom")
		}
	}

	return err
}
