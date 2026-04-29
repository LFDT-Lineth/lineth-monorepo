package controller

import (
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/dlclark/regexp2"
)

const (
	jobNameExecution        = "execution"
	jobNameDataAvailability = "compression"
	jobNameAggregation      = "aggregation"
	jobNameInvalidity       = "invalidity"
)

// JobDefinition represents a collection of static parameters allowing to define
// a job.
type JobDefinition struct {

	// Parameters for the job definition provided by the user
	RequestsRootDir string

	// Name of the job
	Name string

	// The regexp to use to match input files. For instance,
	//
	// 	`^\d+-\d+-etv0.1.2-stv\d.\d.\d-getZkProof.json$`
	//
	// Will tell the controller to accept any version of the state-manager
	// but to only accept execution trace. The regexp should always start "^"
	// and end with "$" otherwise you are going to match in-progress files.
	//
	InputFileRegexp *regexp2.Regexp

	// Template to use to generate the output file. The template should have the
	// form of a go template. For instance,
	//
	// 	`{{.From}}-{{.To}}-pv{{.Version}}-stv{{.Stv}}-etv{{.Etv}}-zkProof.json`
	//
	OutputFileTmpl *template.Template

	// Priority at which this type of job should be processed. The lower the
	// more of a priority.
	//
	// Typically 0 for execution and invalidity, 1 for compression and 2 for aggregation.
	//
	Priority int

	// The associated compiled regexp, this saves on recompiling the regexps
	// everytime we want to use them. If a field is not needed, it can be left
	// at zero.
	ParamsRegexp struct {
		Start       *regexp2.Regexp
		End         *regexp2.Regexp
		Stv         *regexp2.Regexp
		Etv         *regexp2.Regexp
		Cv          *regexp2.Regexp
		ContentHash *regexp2.Regexp
	}

	// Regexp of the failure code so that we can trim it if we want to retry.
	FailureSuffix *regexp2.Regexp
}

// Definition of an execution prover job. The function panics on any error since
// it is called at start up.
func ExecutionDefinition(conf *config.Config) JobDefinition {

	// format the extension part of the regexp if provided
	inpFileExt := ""
	if conf.Execution.CanRunFullLarge {
		inpFileExt = fmt.Sprintf(`\.%v`, config.LargeSuffix)
	}

	return JobDefinition{
		RequestsRootDir: conf.Execution.RequestsRootDir,

		// Name of the job
		Name: jobNameExecution,

		// This will panic at startup if the regexp is invalid
		InputFileRegexp: regexp2.MustCompile(
			fmt.Sprintf(
				`^[0-9]+-[0-9]+(-etv[0-9\.]+)?(-stv[0-9\.]+)?-getZkProof\.json%v(\.failure\.%v_[0-9]+)*$`,
				inpFileExt,
				config.FailSuffix,
			),
			regexp2.None,
		),

		// This will panic at startup if the template is invalid
		OutputFileTmpl: tmplMustCompile(
			"exec-output-file",
			"{{.Start}}-{{.End}}-getZkProof.json",
		),

		// Execution job are at utmost priority
		Priority: 0,

		// Parameters of the regexp, they can loose in the sense that these regexp
		// are only called if the `InputFileRegexp` is matched.
		ParamsRegexp: struct {
			Start       *regexp2.Regexp
			End         *regexp2.Regexp
			Stv         *regexp2.Regexp
			Etv         *regexp2.Regexp
			Cv          *regexp2.Regexp
			ContentHash *regexp2.Regexp
		}{
			// Match a string of digit at the beginning of the line
			Start: regexp2.MustCompile(`^[0-9]+`, regexp2.None),
			// Match a string of digit coming after the first string of digits that
			// initiate the line and followed by a "-"
			End: regexp2.MustCompile(`(?<=^[0-9]+-)[0-9]+`, regexp2.None),
			// Match a sequence of digits and "." comining after (resp.) "etv" and
			// "cv"
			Etv: matchVersionWithPrefix("etv"),
			Stv: matchVersionWithPrefix("stv"),
		},

		FailureSuffix: matchFailureSuffix(config.FailSuffix),
	}
}

// Definition of an execution prover job.
func CompressionDefinition(conf *config.Config) JobDefinition {

	return JobDefinition{
		RequestsRootDir: conf.DataAvailability.RequestsRootDir,

		// Name of the job
		Name: jobNameDataAvailability,

		// This will panic at startup if the regexp is invalid
		InputFileRegexp: regexp2.MustCompile(
			fmt.Sprintf(
				`^[0-9]+-[0-9]+(-bcv[0-9\.]+)?(-ccv[0-9\.]+)?-((0x)?[0-9a-zA-Z]*-)?getZkBlobCompressionProof\.json(\.failure\.%v_[0-9]+)*$`,
				config.FailSuffix,
			),
			regexp2.None,
		),

		// This will panic at startup if the template is invalid
		OutputFileTmpl: tmplMustCompile(
			"compress-output-file",
			"{{.Start}}-{{.End}}-{{.ContentHash}}getZkBlobCompressionProof.json",
		),

		// Compression jobs have secondary priority
		Priority: 1,

		// Parameters of the regexp, they can loose in the sense that these regexp
		// are only called if the `InputFileRegexp` is matched.
		ParamsRegexp: struct {
			Start       *regexp2.Regexp
			End         *regexp2.Regexp
			Stv         *regexp2.Regexp
			Etv         *regexp2.Regexp
			Cv          *regexp2.Regexp
			ContentHash *regexp2.Regexp
		}{
			// Match a string of digit at the beginning of the line
			Start: regexp2.MustCompile(`^[0-9]+`, regexp2.None),
			// Match a string of digit coming after the first string of digits that
			// initiate the line and followed by a "-"
			End: regexp2.MustCompile(`(?<=^[0-9]+-)[0-9]+`, regexp2.None),
			// Match any string containing digits and "." coming after the "cv"
			Cv: matchVersionWithPrefix("cv"),
			// Matches the string between ccv and and getZkBlobCompression
			ContentHash: regexp2.MustCompile(`(?<=ccv[0-9\.]+-)(0x)?[0-9a-zA-Z]+-(?=getZk)`, regexp2.None),
		},

		FailureSuffix: matchFailureSuffix(config.FailSuffix),
	}
}

// Definition of an execution prover job.
func AggregatedDefinition(conf *config.Config) JobDefinition {

	return JobDefinition{
		RequestsRootDir: conf.Aggregation.RequestsRootDir,

		// Name of the job
		Name: jobNameAggregation,

		// This will panic at startup if the regexp is invalid
		InputFileRegexp: regexp2.MustCompile(
			fmt.Sprintf(
				`^[0-9]+-[0-9]+(-[a-fA-F0-9]+)?-getZkAggregatedProof\.json(\.failure\.%v_[0-9]+)*$`,
				config.FailSuffix,
			),
			regexp2.None,
		),

		// This will panic at startup if the template is invalid
		OutputFileTmpl: tmplMustCompile(
			"agreg-output-file",
			"{{.Start}}-{{.End}}-{{.ContentHash}}-getZkAggregatedProof.json",
		),

		// Aggregation prover executes with the lowest priority
		Priority: 2,

		// Parameters of the regexp, they can loose in the sense that these
		// regexp are only called if the `InputFileRegexp` is matched.
		ParamsRegexp: struct {
			Start       *regexp2.Regexp
			End         *regexp2.Regexp
			Stv         *regexp2.Regexp
			Etv         *regexp2.Regexp
			Cv          *regexp2.Regexp
			ContentHash *regexp2.Regexp
		}{
			// Match a string of digit at the beginning of the line
			Start: regexp2.MustCompile(`^[0-9]+`, regexp2.None),
			// Match a string of digit coming after the first string of digits
			// that initiate the line and followed by a "-"
			End: regexp2.MustCompile(`(?<=^[0-9]+-)[0-9]+`, regexp2.None),
			// Match the hexadecimal string that precedes `getZkAggregatedProof`
			ContentHash: regexp2.MustCompile(`(?<=^[0-9]+-[0-9]+-)[a-fA-F0-9]+(?=-getZk)`, regexp2.None),
		},

		FailureSuffix: matchFailureSuffix(config.FailSuffix),
	}
}

// Definition of an invalidity prover job.
func InvalidityDefinition(conf *config.Config) JobDefinition {

	return JobDefinition{
		RequestsRootDir: conf.Invalidity.RequestsRootDir,

		Name: jobNameInvalidity,

		InputFileRegexp: regexp2.MustCompile(
			fmt.Sprintf(
				`^[0-9]+-[0-9]+-getZkInvalidityProof\.json(\.failure\.%v_[0-9]+)*$`,
				config.FailSuffix,
			),
			regexp2.None,
		),

		OutputFileTmpl: tmplMustCompile(
			"invalidity-output-file",
			"{{.Start}}-{{.End}}-getZkInvalidityProof.json",
		),

		// Invalidity proofs have the same priority as execution
		Priority: 0,

		ParamsRegexp: struct {
			Start       *regexp2.Regexp
			End         *regexp2.Regexp
			Stv         *regexp2.Regexp
			Etv         *regexp2.Regexp
			Cv          *regexp2.Regexp
			ContentHash *regexp2.Regexp
		}{
			Start: regexp2.MustCompile(`^[0-9]+`, regexp2.None),
			End:   regexp2.MustCompile(`(?<=^[0-9]+-)[0-9]+`, regexp2.None),
		},

		FailureSuffix: matchFailureSuffix(config.FailSuffix),
	}
}

// Version prefix template
func matchVersionWithPrefix(pre string) *regexp2.Regexp {
	return regexp2.MustCompile(
		fmt.Sprintf(`(?<=%v)[\d\.]+`, pre),
		regexp2.None,
	)
}

// Match the failure code suffix. This string will essentially match all the
// substrints of the form `.failure.code_<X>` so that they can be replaced with
// the empty string.
func matchFailureSuffix(pre string) *regexp2.Regexp {
	return regexp2.MustCompile(
		fmt.Sprintf(`\.failure\.%v_[0-9]+`, pre),
		regexp2.None,
	)
}

// Generates a template or panics
func tmplMustCompile(name, tmpl string) *template.Template {
	res, err := template.New(name).Parse(tmpl)
	if err != nil {
		utils.Panic("could not generate template: %v", err)
	}
	return res
}

func (jd *JobDefinition) dirFrom() string {
	return filepath.Join(jd.RequestsRootDir, config.RequestsFromSubDir)
}

func (jd *JobDefinition) dirDone() string {
	return filepath.Join(jd.RequestsRootDir, config.RequestsDoneSubDir)
}

func (jd *JobDefinition) dirTo() string {
	return filepath.Join(jd.RequestsRootDir, config.RequestsToSubDir)
}
