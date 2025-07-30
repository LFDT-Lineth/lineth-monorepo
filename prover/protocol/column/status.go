package column

import (
	"fmt"
	"strconv"
)

// Status is a tag that we associate to a [github.com/consensys/linea-monorepo/prover/protocol/ifaces.Column]. The status carries
// information about the role of the column in protocol: if it is visible by
// the verifier, whether the column is assigned at compilation-time or at
// runtime, etc...
//
// The default value of Status is [Ignored] which denotes a value that is
// only accessible to the prover and which should not be subjected to
// compilation.
//
// The status assigned to a column can be modified during the compilation. In
// principle, the compilation status should ensure that all columns are either
// [Ignored], [Proof] or [VerifyingKey].
type Status int

const (
	// Ignored denotes a Status indicating that the column should be ignored by
	// the compiler. The main use-case for this is to indicate that the column
	// has already been compiled-out. This happens when a [Committed] column is
	// compiled with the Vortex compiler as the value is then actually committed.
	// Another case is the splitting compiler, which breaks down a column in
	// several segments: in that case the broken down column is replaced by the
	// segments and the rest of the compilation should operate on the segment
	// and not on the original column anymore. The column is still visible to
	// the prover and should still be assigned.
	Ignored Status = iota + 1
	// Committed marks that a [github.com/consensys/linea-monorepo/prover/protocol/ifaces.Column] is to be sent to the oracle,
	// implicitly this is a request for the following steps of the compiler
	// to ensure that the column will be committed to and constitutes a part
	// of the witness of the protocol.
	Committed
	// Proof indicates that the [github.com/consensys/linea-monorepo/prover/protocol/ifaces.Column] should be sent to the verifier.
	// The fact that a step of the compiler marks a column as Proof is not a
	// definitive guarantee that the column will effectively be sent to the
	// verifier. The best example is self-recursion which converts the Proof
	// columns created by the Vortex compiler as Committed back. What is sent
	// to the prover is what is tagged as a proof at the end of the full
	// compilation process.
	Proof
	// Precomputed indicates that the [github.com/consensys/linea-monorepo/prover/protocol/ifaces.Column] is defined offline during
	// the definition or the compilation phase but should not be visible to the
	// verifier and this is an indication that the column should be committed
	// to. An example of such columns are the q_L, q_R, q_M, q_O, q_PI columns
	// defining a Plonk circuit. These columns are known offline but the
	// verifier only interacts with a commitment (or has oracle-access) to these
	// columns.
	Precomputed
	// PublicInput denotes that the column is part of the public inputs of the
	// protocol. Meaning that this is not part of the proof.
	//
	// Deprecated: we don't really use this to create public inputs.
	_
	// VerifyingKey indicates the column is defined offline during the definition
	// of the protocol or the compilation and that the column is directly
	// available to the verifier. It is preferable to avoid tagging large
	// columns as VerifyingKey as this increases the load on the verifier.
	VerifyingKey
	// VerifierDefined indicates that the column is a special type of column
	// that does not need to be assigned as it is constructible by the
	// verifier using data which it already has from other ways. For instance, a
	// column that is constructed from the parameters of an evaluation query or
	// a column that is constructed by stitching together random coins that
	// the verifier sampled through other means.
	VerifierDefined
)

// String returns a string representation of the status. Useful for logging or
// for debugging.
func (s Status) String() string {
	switch s {
	case Committed:
		return "COMMITTED"
	case Ignored:
		return "IGNORED"
	case Proof:
		return "PROOF"
	case Precomputed:
		return "PRECOMPUTED"
	case VerifyingKey:
		return "VERIFYING_KEY"
	case VerifierDefined:
		return "VERIFIER_DEFINED"
	}
	panic("unknown")
}

// IsPublic returns true if the column is visible to the verifier
func (s Status) IsPublic() bool {
	switch s {
	case Proof, VerifyingKey, VerifierDefined:
		return true
	default:
		return false
	}
}

// MarshalJSON implements [json.Marshaler] directly returning the Itoa of the
// integer.
func (t Status) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(t))), nil
}

// UnmarshalJSON implements [json.Unmarshaler] and directly reuses ParseInt and
// performing validation : only 0 and 1 are acceptable values.
func (t *Status) UnmarshalJSON(b []byte) error {
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse Status as integer: %w, got `%v`", err, string(b))
	}

	if n < 0 || Status(n) > VerifierDefined {
		return fmt.Errorf("could not parse the integer `%v` as Status, must be in range [0, 1]", n)
	}

	*t = Status(n)
	return nil
}
