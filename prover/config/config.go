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
	return newConfigFromFile(path, true)
}

// NewConfigFromFileUnchecked is as [NewConfigFromFile] but does not run
// the config validation. It will return an error if it fails reading
// the file or if the config contains unknown fields.
func NewConfigFromFileUnchecked(path string) (*Config, error) {
	return newConfigFromFile(path, false)
}

// NewConfigFromFile reads the configuration from the given file path and returns a new Config.
// It also sets default value and validate the configuration.
func newConfigFromFile(path string, withValidation bool) (*Config, error) {
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

	if withValidation {
		// Validate the config
		validate := validator.New(validator.WithRequiredStructEnabled())
		if err = validate.RegisterValidation("power_of_2", validateIsPowerOfTwo); err != nil {
			return nil, err
		}

		if err = validate.Struct(cfg); err != nil {
			return nil, err
		}

		// Presence-check base_fee via viper. `required` on a uint rejects 0,
		// which is valid on gasless chains. Keep the error format identical
		// to the tag's output so tooling that parses it is unaffected.
		if !viper.IsSet("layer2.base_fee") {
			return nil, fmt.Errorf("Key: 'Config.Layer2.BaseFee' Error:Field validation for 'BaseFee' failed on the 'required' tag")
		}
	}

	// Ensure cmdTmpl and cmdLargeTmpl are parsed
	cfg.Controller.WorkerCmdTmpl, err = template.New("worker_cmd").Parse(cfg.Controller.WorkerCmd)
	if withValidation && err != nil {
		return nil, fmt.Errorf("failed to parse worker_cmd template: %w", err)
	}
	cfg.Controller.WorkerCmdLargeTmpl, err = template.New("worker_cmd_large").Parse(cfg.Controller.WorkerCmdLarge)
	if withValidation && err != nil {
		return nil, fmt.Errorf("failed to parse worker_cmd_large template: %w", err)
	}

	// Set the logging level
	logrus.SetLevel(logrus.Level(cfg.LogLevel)) // #nosec G115 -- overflow not possible (uint8 -> uint32)

	// Extract the Layer2.MsgSvcContract address from the string
	lsMsgSvcAddress, err := common.NewMixedcaseAddressFromString(cfg.Layer2.MsgSvcContractStr)
	if withValidation && err != nil {
		return nil, fmt.Errorf("failed to extract Layer2.MsgSvcContract address: %w", err)
	}
	cfg.Layer2.MsgSvcContract = lsMsgSvcAddress.Address()

	// Extract the coinbase address from the string
	coinBaseAddr, err := common.NewMixedcaseAddressFromString(cfg.Layer2.CoinBaseStr)
	if withValidation && err != nil {
		return nil, fmt.Errorf("failed to extract Layer2.CoinBase address: %w", err)
	}
	cfg.Layer2.CoinBase = coinBaseAddr.Address()

	// ensure that asset dir / kzgsrs exists using os.Stat
	srsDir := cfg.PathForSRS()
	if _, err := os.Stat(srsDir); withValidation && os.IsNotExist(err) {
		return nil, fmt.Errorf("kzgsrs directory (%s) does not exist: %w", srsDir, err)
	}

	// Note: ChainID and BaseFee are typed as uint, which is at most 64 bits.
	cfg.PublicInputInterconnection.ChainID = uint64(cfg.Layer2.ChainID)
	cfg.PublicInputInterconnection.BaseFee = uint64(cfg.Layer2.BaseFee)
	cfg.PublicInputInterconnection.CoinBase = cfg.Layer2.CoinBase
	cfg.PublicInputInterconnection.L2MsgServiceAddr = cfg.Layer2.MsgSvcContract

	// Pass IsAllowedCircuitID from aggregation config to PI circuit
	cfg.PublicInputInterconnection.IsAllowedCircuitID = cfg.Aggregation.IsAllowedCircuitID

	cfg.TracesLimits.normalizeToLowercase()
	cfg.TracesLimits.sortReverseAlphabetical()

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
	// Environment is informational only — it indicates which environment (e.g. devnet, sepolia, mainnet)
	// the config file targets. It is not used for setup paths or any runtime logic;
	// all environments share the same setup and are differentiated only by layer2 public inputs.
	Environment string `mapstructure:"environment"`

	// TODO @gbotrel define explicitly where we use that and for why;
	// if we supply as is to coordinator in responses, coordinator should parse semver
	Version string `validate:"required,semver"`

	// LogLevel sets the log level for the logger.
	LogLevel logLevel `mapstructure:"log_level" validate:"required,gte=0,lte=6"`

	// AssetsDir stores the root of the directory where the assets are stored (setup) or
	// accessed (prover). The file structure is described in TODO @gbotrel.
	AssetsDir string `mapstructure:"assets_dir"`

	Controller                 Controller
	Execution                  Execution
	DataAvailability           DataAvailability `mapstructure:"data_availability"`
	Aggregation                Aggregation
	PublicInputInterconnection PublicInput `mapstructure:"public_input_interconnection"` // TODO add wizard compilation params
	Debug                      Debug       `mapstructure:"debug"`

	Layer2 struct {
		// ChainID stores the ID of the Linea L2 network to consider.
		ChainID uint `mapstructure:"chain_id" validate:"required"`
		// No validator tag: presence is enforced in newConfigFromFile via
		// viper.IsSet. `required` would reject 0, which is valid on gasless
		// chains where every block has baseFeePerGas=0.
		BaseFee uint `mapstructure:"base_fee"`

		// MsgSvcContractStr stores the unique ID of the Service Contract (SC), that is, it's
		// address, as a string. The Service Contract (SC) is a smart contract that the L2
		// network uses to send messages (i.e., transactions) to the L1 (mainnet).
		// Use this field when you need the ETH address as a string.
		MsgSvcContractStr string `mapstructure:"message_service_contract" validate:"required,eth_addr"`

		// MsgSvcContract stores the unique ID of the Service Contract (SC), as a common.Address.
		MsgSvcContract common.Address `mapstructure:"-"`

		// CoinBaseStr stores the coinbase address of Linea as a string.
		CoinBaseStr string         `mapstructure:"coin_base" validate:"required,eth_addr"`
		CoinBase    common.Address `mapstructure:"-"`
	}

	TracesLimits TracesLimits `mapstructure:"traces_limits" validate:"required"`
}

func (cfg *Config) Logger() *logrus.Logger {
	// TODO @gbotrel revisit.
	return logrus.StandardLogger()
}

// PathForSetup returns the path to the setup directory for the given circuitID.
// e.g. .../prover-assets/0.1.0/execution
func (cfg *Config) PathForSetup(circuitID string) string {
	return path.Join(cfg.AssetsDir, cfg.Version, circuitID)
}

// PathForSRS returns the path to the SRS directory.
func (cfg *Config) PathForSRS() string {
	return path.Join(cfg.AssetsDir, "kzgsrs")
}

// ExecutionCircuitBin returns whether the execution circuit binary should be used
// and the path to it.
func (cfg *Config) ExecutionCircuitBin(circuitID string) (enabled bool, path string) {
	if !cfg.Execution.Serialization {
		return false, ""
	}
	return true, filepath.Join(cfg.PathForSetup(circuitID), ExecutionCircuitBinFileName)
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

	// The number of seconds infra (AWS) waits before reclaiming a spot instance
	SpotInstanceReclaimTime int `mapstructure:"spot_instance_reclaim_time_seconds"`

	// The number of seconds the controller should wait before killing a worker after receiving a SIGTERM
	TerminationGracePeriod int `mapstructure:"termination_grace_period_seconds"`

	// defaults to true; the controller will not pick associated jobs if false.
	EnableExecution        bool `mapstructure:"enable_execution"`
	EnableDataAvailability bool `mapstructure:"enable_data_availability"`
	EnableAggregation      bool `mapstructure:"enable_aggregation"`

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
	ProverMode ProverMode `mapstructure:"prover_mode" validate:"required,oneof=dev partial full proofless bench check-only limitless"`

	// CanRunFullLarge indicates whether the prover is running on a large machine (and can run full large traces).
	CanRunFullLarge bool `mapstructure:"can_run_full_large"`

	// ConflatedTracesDir stores the directory where the conflation traces are stored.
	ConflatedTracesDir string `mapstructure:"conflated_traces_dir" validate:"required"`

	// IgnoreCompatiblityCheck indicates whether to ignore constaints version checking between
	// trace files and zkevm.bin constraint files. Specifically, this check ensures that the zkevm.bin file
	// used within the prover was generated from the same commit of linea-constraints as the generated lt trace file.
	// Set this to true to disable compatibility checks (default: false).
	IgnoreCompatibilityCheck bool `mapstructure:"ignore_compatibility_check"`

	// LimitlessWithDebug is only looked at when the limitless prover is
	// activated. When set to true, the limitless prover will only run in
	// debug mode and not produce any proof. This is useful to investigate
	// bugs in the limitless prover. The field is optional and defaults to
	// false.
	LimitlessWithDebug bool `mapstructure:"limitless_with_debug"`

	// KeepTraceUntil is an optional parameter (default to 0) that indicates a
	// maximum block height under which the prover will *not* delete the
	// execution traces from EFS. The block height that is compared to this
	// value is the "end" block of the request range.
	KeepTracesUntilBlock int `mapstructure:"keep_traces_until_block"`

	// Serialization enables loading the inner circuit (wizard IOP) from a
	// serialized file on disk via memory-mapping, instead of compiling it
	// at proving time. The file is produced during setup by protocol/serde.
	Serialization bool `mapstructure:"serialization"`
}

type DataAvailability struct {
	WithRequestDir `mapstructure:",squash"`

	// ProverMode stores the kind of prover to use.
	ProverMode ProverMode `mapstructure:"prover_mode" validate:"required,oneof=dev full"`

	// MaxNbBatches is the maximum number of execution batches allowed in a single DA proof.
	MaxNbBatches int `mapstructure:"max_nb_batches" validate:"required,number,gt=0"`

	// MaxUncompressedNbBytes is the maximum number of bytes in the uncompressed blob payload.
	MaxUncompressedNbBytes int `mapstructure:"max_uncompressed_nb_bytes" validate:"required,gt=0"`

	// DictPaths points to dictionaries used for decompressing executions data stored in the DA layer.
	DictPaths []string `mapstructure:"dict_paths"`

	// DictNbBytes number of bytes in the prover dictionary
	DictNbBytes int `mapstructure:"dict_nb_bytes" validate:"number,gt=0"`
}

type Aggregation struct {
	WithRequestDir `mapstructure:",squash"`

	// ProverMode stores the kind of prover to use.
	ProverMode ProverMode `mapstructure:"prover_mode" validate:"required,oneof=dev full"`

	// Number of proofs that are supported by the aggregation circuit.
	NumProofs []int `mapstructure:"num_proofs" validate:"required,dive,gt=0,number"`

	// IsAllowedCircuitID is a bitmask encoding which circuit IDs are allowed.
	// Bit i (LSb to MSb) indicates whether circuit ID i is allowed.
	// This should be computed based on the environment (testnet vs mainnet).
	// Use circuits.GlobalCircuitIDMapping to set the appropriate bits.
	IsAllowedCircuitID uint64 `mapstructure:"is_allowed_circuit_id" validate:"required"`

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
	MaxNbDataAvailability int `mapstructure:"max_nb_data_availability" validate:"gte=0"`
	MaxNbExecution        int `mapstructure:"max_nb_execution" validate:"gte=0"`
	MaxNbCircuits         int `mapstructure:"max_nb_circuits" validate:"gte=0"` // if not set, will be set to MaxNbDA + MaxNbExecution
	ExecutionMaxNbMsg     int `mapstructure:"execution_max_nb_msg" validate:"gte=0"`
	L2MsgMerkleDepth      int `mapstructure:"l2_msg_merkle_depth" validate:"gte=0"`
	L2MsgMaxNbMerkle      int `mapstructure:"l2_msg_max_nb_merkle" validate:"gte=0"` // if not explicitly provided (i.e. non-positive) it will be set to maximum

	// not serialized

	MockKeccakWizard   bool           // for testing purposes only
	ChainID            uint64         // duplicate from Config.Layer2
	BaseFee            uint64         // duplicate from Config.Layer2
	CoinBase           common.Address // duplicate from Config.Layer2
	L2MsgServiceAddr   common.Address // duplicate from Config.Layer2
	IsAllowedCircuitID uint64         // duplicate from Config.Aggregation, bitmask of allowed circuit IDs
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
// loaded from paths specified in [DataAvailability.DictPaths].
// If no such path is provided, it loads one from the
// prover assets path depending on the provided circuitID.
func (cfg *Config) BlobDecompressionDictStore(circuitID string) dictionary.Store {

	paths := cfg.DataAvailability.DictPaths
	if len(paths) == 0 {
		paths = []string{filepath.Join(cfg.PathForSetup(circuitID), DefaultDictionaryFileName)}
	}

	return dictionary.NewStore(paths...)
}
