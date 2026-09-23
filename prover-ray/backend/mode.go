package backend

// ProverMode selects how a Job is proved. Dev modes skip real proving; full is
// the real path.
type ProverMode string

const (
	// ProverModeDevMock runs no guest and returns a placeholder response. Needs
	// no guest ELF or circuit bin.
	ProverModeDevMock ProverMode = "dev-mock"
	// ProverModeDevNative fills fields from a native guest build; not wired yet.
	ProverModeDevNative ProverMode = "dev-native"
	// ProverModeDevZkVM runs the guest under ZkC Execute and cross-checks it
	// against the native fields; not wired yet.
	ProverModeDevZkVM ProverMode = "dev-zkvm"
	// ProverModePartial traces and checks the trace against the constraints, no
	// proof; memory-gated.
	ProverModePartial ProverMode = "partial"
	// ProverModeFull is the real STARK-proving path; blocked.
	ProverModeFull ProverMode = "full"
)

// Valid reports whether m is a known mode.
func (m ProverMode) Valid() bool {
	switch m {
	case ProverModeDevMock, ProverModeDevNative, ProverModeDevZkVM, ProverModePartial, ProverModeFull:
		return true
	default:
		return false
	}
}

// IsDev reports whether m is a non-production mode.
func (m ProverMode) IsDev() bool {
	switch m {
	case ProverModeDevMock, ProverModeDevNative, ProverModeDevZkVM:
		return true
	default:
		return false
	}
}

// needsArtifacts reports whether New must load the circuit bin and guest ELF.
func (m ProverMode) needsArtifacts() bool {
	return m != ProverModeDevMock
}

// devMarkerProof is the provisional dev-mode proof: a marker, not a real proof.
func devMarkerProof(m ProverMode) []byte {
	return []byte("dev-proof:" + string(m))
}
