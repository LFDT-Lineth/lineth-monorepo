package statemanager

import (
	"errors"
	"fmt"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// Checks a list of trace relating to an account the slice is expected to
// consist in several sequences of ST access delimited by WS accesses on
// the right. The exact pattern is inspected prior to calling this function
// so any irregularity in the pattern yields a panic.
func checkProofsForAccount(traces []DecodedTrace) error {

	curr := []DecodedTrace{}

	for _, trace := range traces {
		// push the current splice
		if trace.isWorldState() {
			// If the length is zero. There is nothing to check in the storage
			// trie. so we can just move forward. It can happen if two WS access
			// for the same account arises consecutively. For instance a deletion
			// followed by an insertion without any access in the newly created
			// account (even though that's an edge-case)
			if len(curr) > 0 {

				logrus.Tracef("checking proof for segment of length %v for account %v", len(curr), trace.Location)
				old, new, err := checkSpliceST(curr)
				if err != nil {
					return err
				}

				switch t := trace.Underlying.(type) {
				case ReadNonZeroTraceWS:
					// We already audited that a ReadNoZero can only follow a sequence
					// of read-only accesses. This contradicts old != new hence we
					// panic because we already checked that this was not possible.
					if old != new {
						panic("read but the account state has been updated")
					}
					// Also the "read account storage root" should be consistent with the
					// one we recovered from the ST traces
					if t.Value.StorageRoot != old {
						return fmt.Errorf("read account has an inconsistent storage root with the one recovered from the ST traces")
					}
				case ReadZeroTraceWS:
					// The len(curr) > 0 and the fact that we previously enforces a
					// pattern that says that if we have a ReadZero for an account
					// then it excludes all other possible accesses.
					panic("read zero but there are ST accesses")
				case InsertionTraceWS:
					// The initial value should be the empty tree
					if old != ZKHASH_EMPTY_STORAGE {
						return fmt.Errorf("sequence of storage access followed by an insertion, but the old (%v) was not the empty storage root (%v)", old.Hex(), ZKHASH_EMPTY_STORAGE.Hex())
					}
					// The recovered new root hash should be consistent with the one
					// inserted
					if t.Val.StorageRoot != new {
						return fmt.Errorf(
							"inserted account has an inconsistent root hash %v with the one recovered from the storage accesses %v",
							t.Val.StorageRoot.Hex(), new.Hex())
					}
				case UpdateTraceWS:
					// The recovered old and new root hashes should be consistent with
					// the updated account
					if t.OldValue.StorageRoot != old || t.NewValue.StorageRoot != new {
						return fmt.Errorf("updated account has inconsistent old and/or new root hash")
					}
				case DeletionTraceWS:
					// The ST traces are assumed to be read-only. The pattern matching
					// procedure checked this already. That's why it is a panic.
					if old != new {
						panic("deletion : but the traces updated the root hash")
					}
					// alex: The deletion trace does not **always** contain the deleted
					// account itself. As a consequence, we cannot check that the deleted
					// account had a consistent storage trie compared to what is recovered
					// in the traces (old).
					if (t.DeletedValue != Account{}) {
						if t.DeletedValue.StorageRoot != old {
							return fmt.Errorf("deletion : but the deleted account storage hash does not match the old value")
						}
					}
				}

				// And start a new splice
				curr = []DecodedTrace{}
			}
			continue
		}
		// else push the trace onto the current splice
		curr = append(curr, trace)
	}

	// sanity-check: the traces should always end with a WS. This is enforced by the pattern inspection
	// that's why it's a panic.
	if len(curr) > 0 {
		panic("the traces do not end with a WS traces, but this was checked by pattern inspection already")
	}

	return nil
}

func checkProofsWorldState(traces []DecodedTrace) (oldRootHash, newRootHash Digest, err error) {

	if len(traces) == 0 {
		panic("unexpected empty slice")
	}

	// uses the first trace to bootstrap the verifier
	vs := bootstrapVerifierStateFromWS(traces[0])
	oldRootHash = vs.TopRoot()

	for i, trace := range traces {

		if i < len(traces)-1 {

			var (
				currHKey = traces[i].Underlying.HKey()
				nextHKey = traces[i+1].Underlying.HKey()
			)

			if currHKey.Cmp(nextHKey) > 0 {
				err = errors.Join(
					err,
					fmt.Errorf("the account segment are not well-ordered `%x` >= `%x`", currHKey, nextHKey),
				)
			}
		}

		switch t := trace.Underlying.(type) {
		case ReadNonZeroTraceWS:
			// this update the storage verifier if this passes
			if errA := vs.ReadNonZeroVerify(t); errA != nil {
				err = errors.Join(err, fmt.Errorf("error verifying ws trace %v: %w", i, errA))
			}
		case ReadZeroTraceWS:
			// this update the storage verifier if this passes
			if errA := vs.ReadZeroVerify(t); errA != nil {
				err = errors.Join(err, fmt.Errorf("error verifying ws trace %v: %w", i, errA))
			}
		case InsertionTraceWS:
			// this updates the storage verifier if this passes
			if errA := vs.VerifyInsertion(t); errA != nil {
				err = errors.Join(err, fmt.Errorf("error verifying ws trace %v: %w", i, errA))
			}
		case UpdateTraceWS:
			// this updates the storage verifier if this passes
			if errA := vs.UpdateVerify(t); errA != nil {
				err = errors.Join(err, fmt.Errorf("error verifying ws trace %v: %w", i, errA))
			}
		case DeletionTraceWS:
			// this updates the storage verifier if this passes
			if errA := vs.VerifyDeletion(t); errA != nil {
				err = errors.Join(err, fmt.Errorf("error verifying ws trace %v: %w", i, errA))
			}
		default:
			utils.Panic("unexpected trace type: %T", trace.Underlying)
		}
	}

	if err != nil {
		return Digest{}, Digest{}, err
	}

	newRootHash = vs.TopRoot()
	return oldRootHash, newRootHash, nil
}

// plays the trace verification and in case of success. Return a recovered initial
// and final root hash.
func checkSpliceST(traces []DecodedTrace) (oldRootHash, newRootHash Digest, err error) {

	if len(traces) == 0 {
		panic("unexpected empty slice")
	}

	// what we return in case of error
	digestErr := Digest{}

	// uses the first trace to bootstrap the verifier
	vs := bootstrapVerifierStateFromST(traces[0])
	oldRootHash = vs.TopRoot()

	for _, trace := range traces {
		switch t := trace.Underlying.(type) {
		case ReadNonZeroTraceST:
			// this update the storage verifier if this passes
			if err := vs.ReadNonZeroVerify(t); err != nil {
				return digestErr, digestErr, err
			}
		case ReadZeroTraceST:
			// this update the storage verifier if this passes
			if err := vs.ReadZeroVerify(t); err != nil {
				return digestErr, digestErr, err
			}
		case InsertionTraceST:
			// this updates the storage verifier if this passes
			if err := vs.VerifyInsertion(t); err != nil {
				return digestErr, digestErr, err
			}
		case UpdateTraceST:
			// this updates the storage verifier if this passes
			if err := vs.UpdateVerify(t); err != nil {
				return digestErr, digestErr, err
			}
		case DeletionTraceST:
			// this updates the storage verifier if this passes
			if err := vs.VerifyDeletion(t); err != nil {
				return digestErr, digestErr, err
			}
		default:
			utils.Panic("unexpected trace type: %T", trace.Underlying)
		}
	}

	newRootHash = vs.TopRoot()
	return oldRootHash, newRootHash, nil
}

func bootstrapVerifierStateFromST(trace DecodedTrace) (vs StorageVerifier) {

	vs = StorageVerifier{
		Location: trace.Location,
	}

	switch t := trace.Underlying.(type) {
	case ReadNonZeroTraceST:
		vs.NextFreeNode = int64(t.NextFreeNode)
		vs.SubTreeRoot = t.SubRoot
	case ReadZeroTraceST:
		vs.NextFreeNode = int64(t.NextFreeNode)
		vs.SubTreeRoot = t.SubRoot
	case InsertionTraceST:
		// because the operation increase the nextFreeNode flag
		vs.NextFreeNode = int64(t.NewNextFreeNode) - 1
		vs.SubTreeRoot = t.OldSubRoot
	case UpdateTraceST:
		vs.NextFreeNode = int64(t.NewNextFreeNode)
		vs.SubTreeRoot = t.OldSubRoot
	case DeletionTraceST:
		vs.NextFreeNode = int64(t.NewNextFreeNode)
		vs.SubTreeRoot = t.OldSubRoot
	default:
		utils.Panic("unexpected type %T", t)
	}

	return vs
}

func bootstrapVerifierStateFromWS(trace DecodedTrace) (vs AccountVerifier) {

	vs = AccountVerifier{
		Location: trace.Location,
	}

	switch t := trace.Underlying.(type) {
	case ReadNonZeroTraceWS:
		vs.NextFreeNode = int64(t.NextFreeNode)
		vs.SubTreeRoot = t.SubRoot
	case ReadZeroTraceWS:
		vs.NextFreeNode = int64(t.NextFreeNode)
		vs.SubTreeRoot = t.SubRoot
	case InsertionTraceWS:
		// because the operation increase the nextFreeNode flag
		vs.NextFreeNode = int64(t.NewNextFreeNode) - 1
		vs.SubTreeRoot = t.OldSubRoot
	case UpdateTraceWS:
		vs.NextFreeNode = int64(t.NewNextFreeNode)
		vs.SubTreeRoot = t.OldSubRoot
	case DeletionTraceWS:
		vs.NextFreeNode = int64(t.NewNextFreeNode)
		vs.SubTreeRoot = t.OldSubRoot
	default:
		utils.Panic("unexpected type %T", t)
	}

	return vs
}
