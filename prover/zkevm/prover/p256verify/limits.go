package p256verify

// Limits defines limits for P256Verify module.
type Limits struct {
	// LimitCalls is the total number of P256 verifications that can be
	// performed across all P256 verification circuits.
	LimitCalls int

	// NbInputInstances is the number of P256 input instances per a single
	// verification circuit. gnark circuit size approximately 709k constraints
	// and in Plonk-in-Wizard (with externalized range checks) approximately
	// 183k rows per instance.
	NbInputInstances int
}
