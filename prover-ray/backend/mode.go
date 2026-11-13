package backend

// ProverMode selects how a Job is proved. Dev modes skip real proving; full is
// the real path.
type ProverMode string

const (
	// ProverModeDevMock runs no guest and returns a placeholder response. Needs
	// no guest ELF or circuit bin.
	ProverModeDevMock ProverMode = "dev-mock"
	// ProverModeDevZkVM runs the guest under ZkC Execute for its real public
	// inputs (via the native runner) and cross-checks the guest's commitment
	// against that runner.
	ProverModeDevZkVM ProverMode = "dev-zkvm"
	// ProverModeDevPartial runs the guest, checks its trace against every
	// constraint and returns the public inputs from the guest output, with a
	// devmode proof in place of a real one.
	ProverModeDevPartial ProverMode = "dev-partial"
	// ProverModeFull is the real STARK-proving path; blocked.
	ProverModeFull ProverMode = "full"
)

// Valid reports whether m is a known mode.
func (m ProverMode) Valid() bool {
	switch m {
	case ProverModeDevMock, ProverModeDevZkVM, ProverModeDevPartial, ProverModeFull:
		return true
	default:
		return false
	}
}

// IsDev reports whether m is a non-production mode.
func (m ProverMode) IsDev() bool {
	switch m {
	case ProverModeDevMock, ProverModeDevZkVM, ProverModeDevPartial:
		return true
	default:
		return false
	}
}

// needsArtifacts reports whether New must load ZkC artifacts (the circuit bin
// and guest ELF). Only dev-mock runs no guest at all.
func (m ProverMode) needsArtifacts() bool {
	switch m {
	case ProverModeDevMock:
		return false
	default:
		return true
	}
}

// DevMarkerProof is the provisional dev-mode proof: a marker, not a real proof.
func DevMarkerProof(m ProverMode) []byte {
	return []byte("dev-proof:" + string(m))
}
