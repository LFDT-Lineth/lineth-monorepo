package config

// TODO @gbotrel don't think these should be defined here, to refactor.

// logLevel defines is a type representing different logging levels.
type logLevel uint8

const (
	// Possible levels of logging.
	_logTrace logLevel = iota + 1
	_logDebug
	_logInfo
	_logWarn
	_logError
	_logFatal
)

type ProverMode string

const (
	ProverModeDev       ProverMode = "dev"
	ProverModePartial   ProverMode = "partial"
	ProverModeFull      ProverMode = "full"
	ProverModeLimitless ProverMode = "limitless"
	ProverModeProofless ProverMode = "proofless"
	// ProverModeBench is used to only run the inner-proof. This is convenient
	// in a context where it is simpler to not have to deal with the setup.
	ProverModeBench ProverMode = "bench"
	// ProverModeCheckOnly is used to test the constraints of the whole system
	ProverModeCheckOnly  ProverMode = "check-only"
	ProverModeEncodeOnly ProverMode = "encode-only"
)
