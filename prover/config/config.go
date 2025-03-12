package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"text/template"
	"time"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// NewConfigFromFile reads the configuration from the given file path and returns a new Config.
// It also sets default value and validate the configuration.
func NewConfigFromFile(path string) (*Config, error) {
	viper.SetConfigFile(path)

	// Parse the config
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	// Set the default values
	setDefaultValues()

	// Unmarshal the config; note that UnmarshalExact will error if there are any fields in the config
	// that are not present in the struct.
	var cfg Config
	err = viper.UnmarshalExact(&cfg)
	if err != nil {
		return nil, err
	}

	// Validate the config
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err = validate.RegisterValidation("power_of_2", validateIsPowerOfTwo); err != nil {
		return nil, err
	}

	if err = validate.Struct(cfg); err != nil {
		return nil, err
	}

	// Ensure cmdTmpl and cmdLargeTmpl are parsed
	cfg.Controller.WorkerCmdTmpl, err = template.New("worker_cmd").Parse(cfg.Controller.WorkerCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse worker_cmd template: %w", err)
	}
	cfg.Controller.WorkerCmdLargeTmpl, err = template.New("worker_cmd_large").Parse(cfg.Controller.WorkerCmdLarge)
	if err != nil {
		return nil, fmt.Errorf("failed to parse worker_cmd_large template: %w", err)
	}

	// Set the logging level
	logrus.SetLevel(logrus.Level(cfg.LogLevel)) // #nosec G115 -- overflow not possible (uint8 -> uint32)

	// Extract the Layer2.MsgSvcContract address from the string
	addr, err := common.NewMixedcaseAddressFromString(cfg.Layer2.MsgSvcContractStr)
	if err != nil {
		return nil, fmt.Errorf("failed to extract Layer2.MsgSvcContract address: %w", err)
	}
	cfg.Layer2.MsgSvcContract = addr.Address()

	// ensure that asset dir / kzgsrs exists using os.Stat
	srsDir := cfg.PathForSRS()
	if _, err := os.Stat(srsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("kzgsrs directory (%s) does not exist: %w", srsDir, err)
	}

	// duplicate L2 hardcoded values for PI
	cfg.PublicInputInterconnection.ChainID = uint64(cfg.Layer2.ChainID)
	cfg.PublicInputInterconnection.L2MsgServiceAddr = cfg.Layer2.MsgSvcContract

	return &cfg, nil
}

// validateIsPowerOfTwo implements validator.Func
func validateIsPowerOfTwo(f validator.FieldLevel) bool {
	if !f.Field().CanInt() {
		return false
	}
	n := f.Field().Int()
	return n > 0 && (n&(n-1)) == 0
}

// TODO @gbotrel add viper hook to decode custom types (instead of having duplicate string and custom type.)

type Config struct {
	// Environment stores the environment in which the application is running.
	// It enables us have a clear domain separation for generated assets.
	Environment string `validate:"required,oneof=mainnet sepolia devnet integration-development integration-full integration-benchmark"`

	// TODO @gbotrel define explicitly where we use that and for why;
	// if we supply as is to coordinator in responses, coordinator should parse semver
	Version string `validate:"required,semver"`

	// LogLevel sets the log level for the logger.
	LogLevel logLevel `mapstructure:"log_level" validate:"required,gte=0,lte=6"`

	// AssetsDir stores the root of the directory where the assets are stored (setup) or
	// accessed (prover). The file structure is described in TODO @gbotrel.
	AssetsDir string `mapstructure:"assets_dir" validate:"required,dir"`

	Controller                 Controller
	Execution                  Execution
	BlobDecompression          BlobDecompression `mapstructure:"blob_decompression"`
	Aggregation                Aggregation
	PublicInputInterconnection PublicInput `mapstructure:"public_input_interconnection"` // TODO add wizard compilation params
	Debug                      Debug       `mapstructure:"debug"`

	Layer2 struct {
		// ChainID stores the ID of the Linea L2 network to consider.
		ChainID uint `mapstructure:"chain_id" validate:"required"`

		// MsgSvcContractStr stores the unique ID of the Service Contract (SC), that is, it's
		// address, as a string. The Service Contract (SC) is a smart contract that the L2
		// network uses to send messages (i.e., transactions) to the L1 (mainnet).
		// Use this field when you need the ETH address as a string.
		MsgSvcContractStr string `mapstructure:"message_service_contract" validate:"required,eth_addr"`

		// MsgSvcContract stores the unique ID of the Service Contract (SC), as a common.Address.
		MsgSvcContract common.Address `mapstructure:"-"`
	}

	TracesLimits      TracesLimits `mapstructure:"traces_limits" validate:"required"`
	TracesLimitsLarge TracesLimits `mapstructure:"traces_limits_large" validate:"required"`
}

func (cfg *Config) Logger() *logrus.Logger {
	// TODO @gbotrel revisit.
	return logrus.StandardLogger()
}

// PathForSetup returns the path to the setup directory for the given circuitID.
// e.g. .../prover-assets/0.1.0/mainnet/execution
func (cfg *Config) PathForSetup(circuitID string) string {
	return path.Join(cfg.AssetsDir, cfg.Version, cfg.Environment, circuitID)
}

// PathForSRS returns the path to the SRS directory.
func (cfg *Config) PathForSRS() string {
	return path.Join(cfg.AssetsDir, "kzgsrs")
}

type Controller struct {
	// The unique id of this process. Must be unique between all workers. This
	// field is not to be populated by the toml configuration file. It is to be
	// through an environment variable.
	LocalID string

	// Prometheus stores the configuration for the Prometheus metrics server.
	Prometheus Prometheus

	// The delays at which we retry when we find no files in the queue. If this
	// is set to [0, 1, 2, 3, 4, 5]. It will retry after 0 sec the first time it
	// cannot find a file in the queue, 1 sec the second time and so on. Once it
	// reaches the final value it keeps it as a final retry delay.
	RetryDelays []int `mapstructure:"retry_delays"`

	// List of exit codes for which the job will put back the job to be reexecuted in large mode.
	DeferToOtherLargeCodes []int `mapstructure:"defer_to_other_large_codes"`

	// List of exit codes for which the job will retry in large mode
	RetryLocallyWithLargeCodes []int `mapstructure:"retry_locally_with_large_codes"`

	// defaults to true; the controller will not pick associated jobs if false.
	EnableExecution         bool `mapstructure:"enable_execution"`
	EnableBlobDecompression bool `mapstructure:"enable_blob_decompression"`
	EnableAggregation       bool `mapstructure:"enable_aggregation"`

	// TODO @gbotrel the only reason we keep these is for test purposes; default value is fine,
	// we should remove them from here for readability.
	WorkerCmd          string             `mapstructure:"worker_cmd_tmpl"`
	WorkerCmdLarge     string             `mapstructure:"worker_cmd_large_tmpl"`
	WorkerCmdTmpl      *template.Template `mapstructure:"-"`
	WorkerCmdLargeTmpl *template.Template `mapstructure:"-"`
}

type Prometheus struct {
	Enabled bool
	// The underlying implementation defaults to :9090.
	Port int
	// The default implementation default to /metrics. The route should be
	// prefixed with a "/". If it is not, the underlying implementation will
	// assign it.
	Route string
}

type Execution struct {
	WithRequestDir `mapstructure:",squash"`

	// ProverMode stores the kind of prover to use.
	ProverMode ProverMode `mapstructure:"prover_mode" validate:"required,oneof=dev partial full proofless bench check-only"`

	// CanRunFullLarge indicates whether the prover is running on a large machine (and can run full large traces).
	CanRunFullLarge bool `mapstructure:"can_run_full_large"`

	// ConflatedTracesDir stores the directory where the conflation traces are stored.
	ConflatedTracesDir string `mapstructure:"conflated_traces_dir" validate:"required"`

	// IgnoreCompatiblityCheck indicates whether to ignore constaints version checking between
	// trace files and zkevm.bin constraint files. Specifically, this check ensures that the zkevm.bin file
	// used within the prover was generated from the same commit of linea-constraints as the generated lt trace file.
	// Set this to true to disable compatibility checks (default: false).
	IgnoreCompatibilityCheck bool `mapstructure:"ignore_compatibility_check"`
}

type BlobDecompression struct {
	WithRequestDir `mapstructure:",squash"`

	// ProverMode stores the kind of prover to use.
	ProverMode ProverMode `mapstructure:"prover_mode" validate:"required,oneof=dev full"`

	// DictPaths is an optional parameters allowing the user to specify explicitly
	// where to look for the compression dictionaries. If the input is not provided
	// then the dictionary will be fetched in <assets_dir>/<version>/<circuitID>/compression_dict.bin.
	//
	// We stress that the feature should not be used in production and should
	// only be used in E2E testing context. TODO @Tabaie @alexandre.belling revise this warning, seems to no longer apply
	DictPaths []string `mapstructure:"dict_paths"`
}

type Aggregation struct {
	WithRequestDir `mapstructure:",squash"`

	// ProverMode stores the kind of prover to use.
	ProverMode ProverMode `mapstructure:"prover_mode" validate:"required,oneof=dev full"`

	// Number of proofs that are supported by the aggregation circuit.
	NumProofs []int `mapstructure:"num_proofs" validate:"required,dive,gt=0,number"`

	// AllowedInputs determines the "inner" plonk circuits the "outer" aggregation circuit can aggregate.
	// Order matters.
	AllowedInputs []string `mapstructure:"allowed_inputs" validate:"required,dive,oneof=execution-dummy execution execution-large blob-decompression-dummy blob-decompression-v0 blob-decompression-v1 emulation-dummy aggregation emulation public-input-interconnection"`

	// note @gbotrel keeping that around in case we need to support two emulation contract
	// during a migration.
	// Verifier ID to assign to the proof once generated. It will be used
	// by the L1 contracts to determine which solidity Plonk verifier
	// contract should be used to verify the proof.
	VerifierID int `mapstructure:"verifier_id" validate:"gte=0,number"`
}

type WithRequestDir struct {
	RequestsRootDir string `mapstructure:"requests_root_dir" validate:"required"`
}

func (cfg *WithRequestDir) DirFrom() string {
	return path.Join(cfg.RequestsRootDir, RequestsFromSubDir)
}

func (cfg *WithRequestDir) DirTo() string {
	return path.Join(cfg.RequestsRootDir, RequestsToSubDir)
}

func (cfg *WithRequestDir) DirDone() string {
	return path.Join(cfg.RequestsRootDir, RequestsDoneSubDir)
}

type PublicInput struct {
	MaxNbDecompression int `mapstructure:"max_nb_decompression" validate:"gte=0"`
	MaxNbExecution     int `mapstructure:"max_nb_execution" validate:"gte=0"`
	MaxNbCircuits      int `mapstructure:"max_nb_circuits" validate:"gte=0"` // if not set, will be set to MaxNbDecompression + MaxNbExecution
	ExecutionMaxNbMsg  int `mapstructure:"execution_max_nb_msg" validate:"gte=0"`
	L2MsgMerkleDepth   int `mapstructure:"l2_msg_merkle_depth" validate:"gte=0"`
	L2MsgMaxNbMerkle   int `mapstructure:"l2_msg_max_nb_merkle" validate:"gte=0"` // if not explicitly provided (i.e. non-positive) it will be set to maximum

	// not serialized

	MockKeccakWizard bool           // for testing purposes only
	ChainID          uint64         // duplicate from Config
	L2MsgServiceAddr common.Address // duplicate from Config
}

type Debug struct {
	// Profiling indicates whether we want to generate profiles using the [runtime/pprof] pkg.
	// Profiles the entire execution proof request-response lifecycle
	Profiling bool `mapstructure:"profiling"`

	// Tracing indicates whether we want to generate traces using the [runtime/trace] pkg.
	// Traces can later be read using the `go tool trace` command.
	Tracing bool `mapstructure:"tracing"`

	// PerformanceMonitor indicates if we want to profile any specific prover steps or rounds
	// This is basically profiling at a granular level
	PerformanceMonitor PerformanceMonitor `mapstructure:"performance_monitor"`
}

type PerformanceMonitor struct {
	Active         bool          `mapstructure:"active"`
	SampleDuration time.Duration `mapstructure:"sample_duration"`
	ProfileDir     string        `mapstructure:"profile_dir"`
	Profile        string        `mapstructure:"profile" validate:"oneof=prover-steps prover-rounds all"`
}

// BlobDecompressionDictStore returns a decompression dictionary store
// loaded from paths specified in [BlobDecompression.DictPaths].
// If no such path is provided, it loads one from the
// prover assets path depending on the provided circuitID.
func (cfg *Config) BlobDecompressionDictStore(circuitID string) dictionary.Store {

	paths := cfg.BlobDecompression.DictPaths
	if len(paths) == 0 {
		paths = []string{filepath.Join(cfg.PathForSetup(circuitID), DefaultDictionaryFileName)}
	}

	return dictionary.NewStore(paths...)
}
