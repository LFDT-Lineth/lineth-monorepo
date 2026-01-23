// Library containing utilities exclusively for the master process
package controller

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/consensys/linea-monorepo/prover/cmd/controller/controller/metrics"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

// FsWatcher is a struct who will watch the filesystem and return files as if
// it were a message queue.
type FsWatcher struct {
	// Unique ID of the container. Used to identify the owner of a locked file
	LocalID string
	// List of jobs that we are currently matching
	JobToWatch []JobDefinition
	// Suffix to append to the end of a file
	InProgress string
	// Logger specific to the file watcher
	Logger *logrus.Entry
}

func NewFsWatcher(conf *config.Config) *FsWatcher {
	fs := &FsWatcher{
		LocalID:    conf.Controller.LocalID,
		InProgress: config.InProgressSuffix,
		Logger:     conf.Logger().WithField("component", "filesystem-watcher"),
	}

	if conf.Controller.EnableExecution {
		fs.JobToWatch = append(fs.JobToWatch, ExecutionDefinition(conf))
	}

	if conf.Controller.EnableBlobDecompression {
		fs.JobToWatch = append(fs.JobToWatch, CompressionDefinition(conf))
	}

	if conf.Controller.EnableAggregation {
		fs.JobToWatch = append(fs.JobToWatch, AggregatedDefinition(conf))
	}

	return fs
}

// Returns the list of jobs to perform by priorities. If no
func (fs *FsWatcher) GetBest() (job *Job) {

	// Fetches the full job list from all three directories. The fetching
	// operation will not ignore files if they are not in the expected
	// directory. For instance, if an aggregation file is in the directory
	// supposed to contain only aggregation jobs.
	jobs := []*Job{}

	// If there are no jobs definition to watch
	if len(fs.JobToWatch) == 0 {
		fs.Logger.Errorf("No job definition to watch")
		return nil
	}

	for i := range fs.JobToWatch {
		// Don't try to pass &jdef, where jdef is a loop variable as
		// `for i, jdef := range f.JobToWatch {...}`
		// Because otherwise, it will modify in-place the value jobdefinition
		// of every jobs found so far and they will all be attributed to the
		// last job definition.
		jdef := &fs.JobToWatch[i]
		if err := fs.appendJobFromDef(jdef, &jobs); err != nil {
			fs.Logger.Errorf(
				"Got an error trying to fetch job `%v` from dir %v: %v",
				jdef.Name, jdef.dirFrom(), err,
			)
		}
	}

	if len(jobs) == 0 {
		fs.Logger.Debugf("The queue is empty")
		return nil
	}

	// Sort the jobs by scores in ascending order. Lower scores mean more
	// priority.
	slices.SortStableFunc(jobs, func(a, b *Job) int {
		return a.Score() - b.Score()
	})

	best, success := fs.lockBest(jobs)
	if !success {
		fs.Logger.Infof(
			"Found %v jobs in the queue. They were all locked before we could pick one",
			len(jobs),
		)
		return nil
	}

	return jobs[best]
}

// Returns the best file that we could lock and its position in the slice. If
// everything failed returns 0, false.
func (f *FsWatcher) lockBest(jobs []*Job) (pos int, success bool) {
	for pos := range jobs {
		if f.tryLockFile(jobs[pos]) {
			return pos, true
		}
	}

	return 0, false
}

// Try appending a list of jobs that are parsed from a given directory. An error
// is returned if the function fails to read the directory.
func (fs *FsWatcher) appendJobFromDef(jdef *JobDefinition, jobs *[]*Job) (err error) {

	dirFrom := jdef.dirFrom()
	fs.Logger.Tracef("Seeking jobs for %v in %v", jdef.Name, dirFrom)

	// This will fail if the provided directory is not a directory
	dirents, err := lsname(dirFrom)
	if err != nil {
		return fmt.Errorf("cannot ls `%s` : %v", dirFrom, err)
	}
	numMatched := 0

	// Search and append the valid files into the list.
	for _, dirent := range dirents {

		fs.Logger.Tracef("Examining entry %s in %s", dirFrom, dirent.Name())

		// Ignore directories
		if !dirent.Type().IsRegular() {
			fs.Logger.Debugf("Ignoring directory `%s`", dirent.Name())
			continue
		}

		// Attempt to construct a job from the filename. If the filename is
		// not parseable to the target JobType, it will return an error.
		job, err := NewJob(jdef, dirent.Name())
		if err != nil {
			fs.Logger.Debugf("Found invalid file  `%v` : %v", dirent.Name(), err)
			continue
		}

		// If all the checks passes, we append the filename to the list of the
		// clean ones.
		*jobs = append(*jobs, job)
		numMatched++
	}

	// Pass prometheus metrics
	metrics.CollectFS(jdef.Name, len(dirents), numMatched)

	return nil
}

// Trylock attempts to rename a file by adding an IN_PROGRESS suffix. The lock
// operation is atomic only on Unix systems.
func (fs *FsWatcher) tryLockFile(job *Job) (success bool) {

	dirName := job.Def.dirFrom()
	lockedFile := strings.Join(
		[]string{
			job.OriginalFile,
			fs.InProgress,
			fs.LocalID,
		}, ".")
	old := path.Join(dirName, job.OriginalFile)
	new := path.Join(dirName, lockedFile)
	err := os.Rename(old, new)

	if err != nil {
		// Detect the case where the old file still exists but the new one
		// already exists.
		_, errOld := os.Lstat(old)
		_, errNew := os.Lstat(new)

		if errNew == nil && errOld == nil {
			fs.Logger.Errorf(
				"old file `%v` and new files `%v` both exists",
				old, new,
			)
		}

		fs.Logger.Tracef(
			"could not lock file %v because : %v",
			old, errOld,
		)

		return false
	}

	// Success, write the name of the locked file
	job.LockedFile = lockedFile

	return true
}

// Returns the list of the entries name in the given directory. Returns up to
// `n` result, ignore the errors for each files. Returns an error if it cannot
// open the directory.
func lsname(dirname string) (finfos []fs.DirEntry, err error) {

	// Attempt to open the directory
	dir, err := os.Open(dirname)
	if err != nil {
		return nil, fmt.Errorf("could not open directory %s: %v", dirname, err)
	}
	defer dir.Close()

	// Check if this is a directory and attempt to read all its entries. It will
	// return an EOF if the dir is empty, which is not an error from the
	// perspective of the application. Any other known error would be that
	// file descriptor of the directory is corrupted.
	finfos, err = dir.ReadDir(-1)
	if err == io.EOF {
		return []fs.DirEntry{}, nil
	}

	return finfos, err
}
