package controller

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/dlclark/regexp2"
	"github.com/sirupsen/logrus"
)

// Defines a job to be processed. Jobs are parsed from filenames using regexp.
type Job struct {
	// Configuration parameters relative to the job
	Def *JobDefinition
	// Original name of the file when it was found
	OriginalFile string
	// Name of the locked file. If this value is set, it means that the job
	// was successfully locked.
	LockedFile string
	// Height of the file in the priority queue
	Start int
	End   int

	VersionExecutionTracer string
	VersionStateManager    string
	VersionCompressor      string

	// The hex string of the content hash
	ContentHash string
}

// OutputFileRessouce collects all the data needed to fill the output template
// file.
type OutputFileRessouce struct {
	Job
}

// Parse a filename into a Job. Returns an error if the file does not
// corresponds to the specified template of the job type.
func NewJob(jdef *JobDefinition, filename string) (j *Job, err error) {
	// Validate the filename against the inbound regexp
	if ok, err := jdef.InputFileRegexp.MatchString(filename); !ok || err != nil {
		return nil, fmt.Errorf(
			"filename %v does not match the inbound regexp for `%v` job: `%v. "+
				"Err if any = %v",
			filename, jdef.Name, jdef.InputFileRegexp.String(), err,
		)
	}

	j = &Job{Def: jdef, OriginalFile: filename}
	regs := jdef.ParamsRegexp

	// If the regexps in the job definition are provided, use them to extract
	// the parameters of the job. If one regexp is provided but is invalid, this
	// will panic. That is because since, we already matched the
	// `InputFileRegexp`, we assume that the parameters regexp can only be
	// matched.
	j.Start = intIfRegexpNotNil(regs.Start, filename)
	j.End = intIfRegexpNotNil(regs.End, filename)
	j.VersionCompressor = stringIfRegexpNotNil(regs.Cv, filename)
	j.VersionExecutionTracer = stringIfRegexpNotNil(regs.Etv, filename)
	j.VersionStateManager = stringIfRegexpNotNil(regs.Stv, filename)
	j.ContentHash = stringIfRegexpNotNil(regs.ContentHash, filename)

	return j, nil
}

// Returns the full path to the original file
func (j *Job) OriginalPath() string {
	return filepath.Join(j.Def.dirFrom(), j.OriginalFile)
}

// Returns the full path to the inprogress file
func (j *Job) InProgressPath() string {
	return filepath.Join(j.Def.dirFrom(), j.LockedFile)
}

// Returns the name of the output file for the job
func (j *Job) ResponseFile() (s string, err error) {

	// Run the template
	w := &strings.Builder{}
	err = j.Def.OutputFileTmpl.Execute(w, OutputFileRessouce{
		Job: *j,
	})
	if err != nil {
		return "", err
	}
	s = w.String()

	// Clear the `--` from the response file name. These can happen if an
	// optional field was not provided in the request file and is given in the
	// response.
	s = strings.ReplaceAll(s, "--", "-")

	// Append the dir_to filepath
	s = path.Join(j.Def.dirTo(), s)

	return s, nil
}

// Returns the name of the output file for the job
func (j *Job) TmpResponseFile(c *config.Config) (s string) {
	return path.Join(j.Def.dirTo(), "tmp-response-file."+c.Controller.LocalID+".json")
}

// Returns the name of the input file modified so that it is retried in
// large mode. It fails if the job's definition does not provide a suffix to
// retry in large mode. This is still unexpected because the configuration
// validation ensures that if there is an exit code amenable to defer the job to
// a larger machine, then the suffix must be set. If the status code of the
// prover is zero it will return an error.
func (j *Job) DeferToLargeFile(status Status) (s string, err error) {

	// It's an invariant of the executor to not forget to set the status
	if status.ExitCode == 0 {
		return "", fmt.Errorf(
			"cant defer to large %v, status code was zero",
			j.OriginalFile,
		)
	}

	const suffixLarge = config.LargeSuffix

	// Issue a warning if the file if the files name already contains the
	// suffix. We may be in a situation where the large prover is trying to
	// defer over the same file. It is very likely an error. We will still
	// rename it to "<...>.large.large". That way, the file will not be picked
	// up a second time by the same large prover, creating an infinite retry
	// loop.
	if strings.HasSuffix(j.OriginalFile, suffixLarge) {
		logrus.Warnf(
			"Deferring the large machine but the input file `%v` already has"+
				" the suffix %v. Still renaming it to %v, but it will likely"+
				" not be picked up again",
			j.OriginalFile, suffixLarge, s,
		)
	}

	// Remove the suffix .failure.code_[0-9]+ from all the strings of the input
	// file. That way we do not propagate the previous errors.
	origFile, err := j.Def.FailureSuffix.Replace(j.OriginalFile, "", -1, -1)
	if err != nil {
		// he assumption here is that the above function may return an error
		// but this error can only depend on the regexp, the replacement,
		// the startAt and the count/ Thus, if it fails, the error is
		// unrelated to the input stream, which is the only user-provided
		// parameter.
		panic(err)
	}

	return fmt.Sprintf(
		"%v/%v.%v.failure.%v_%v",
		j.Def.dirFrom(), origFile,
		suffixLarge,
		config.FailSuffix, status.ExitCode,
	), nil
}

// Returns the done file following the jobs status
func (j *Job) DoneFile(status Status) string {

	// Remove the suffix .failure.code_[0-9]+ from all the strings
	origFile, err := j.Def.FailureSuffix.Replace(j.OriginalFile, "", -1, -1)
	if err != nil {
		// he assumption here is that the above function may return an error
		// but this error can only depend on the regexp, the replacement,
		// the startAt and the count/ Thus, if it fails, the error is
		// unrelated to the input stream, which is the only user-provided
		// parameter.
		panic(err)
	}

	if status.ExitCode == CodeSuccess {
		return fmt.Sprintf("%v/%v.%v", j.Def.dirDone(), origFile, config.SuccessSuffix)
	} else {
		return fmt.Sprintf("%v/%v.failure.%v_%v", j.Def.dirDone(), origFile, config.FailSuffix, status.ExitCode)
	}
}

// Returns the score of a JOB. The score is obtained as 100*job.Stop + P, where
// P is 0 for execution and invalidity, 1 for compression and 2 for aggregation.
// The lower the score the higher will be the priority of the job. The 100 value
// is chosen to make the score easy to mentally compute.
func (j *Job) Score() int {
	return 100*j.End + j.Def.Priority
}

// If the regexp is provided and non-nil, return the first match and returns the
// empty string if no match is found. If the regexp is not provided, returns the
// empty strings
func stringIfRegexpNotNil(r *regexp2.Regexp, s string) (res string) {
	if r != nil {
		res, err := r.FindStringMatch(s)
		if err != nil || res == nil {
			return ""
		}
		return res.String()
	}
	return ""
}

// Parse an integer from a string using a regexp if provided. If the regexp is
// non-nil but does not match the string s, the function panics. If the regexp
// is not provided, the function returns 0.
func intIfRegexpNotNil(r *regexp2.Regexp, s string) int {
	// Map the result as an integer
	match := stringIfRegexpNotNil(r, s)
	if len(s) == 0 {
		return 0
	}

	res, err := strconv.Atoi(match)
	if err != nil {
		// If this happens, it means that the provided regexp does not match
		// decimal digits strings.
		panic(err)
	}
	return res
}
